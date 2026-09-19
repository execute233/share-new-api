# 删除 Audio / Rerank / Realtime 协议 — 设计文档

日期：2026-08-16
状态：已批准（用户确认设计方案）

## 背景与目标

项目是 AI API 网关（new-api），对外暴露约 20 种协议接口。用户希望精简，删除以下三个协议及其关联功能：

- **Audio 协议**：`/v1/audio/transcriptions`、`/v1/audio/translations`、`/v1/audio/speech`（STT/whisper/TTS）
- **Rerank 协议**：`/v1/rerank`
- **Realtime 协议**：`/v1/realtime`（WebSocket）

目标：删除上述协议的完整链路（路由 → handler → 适配器实现 → DTO → 计费专属逻辑 → 模型列表死条目 → 前端 UI 入口），同时保留共享基础设施（audio tokens 计费链、gorilla/websocket 依赖）。

## 决策记录

| 决策点 | 结论 |
|---|---|
| Jina 渠道（Type=38，纯 rerank 渠道） | 随 Rerank 协议一并删除（后端适配器 + 前端映射 + i18n） |
| RelayMode 常量（Audio\* 35-37 / Rerank 46 / Realtime 50） | 彻底删除，iota 断号（历史日志 relay_mode 数字语义前移，可接受；前端不消费该数字） |
| 模型列表死条目（tts/whisper/realtime/rerank） | 清理；`gpt-4o-audio-preview` 等经 chat 使用的 audio 模型保留 |
| 文档（docs/learn、README×4、docs/openapi/relay.json） | 本次不更新 |

## 删除范围

### 1. 整文件删除

| 文件 | 说明 |
|---|---|
| `relay/audio_handler.go` | Audio 入口 handler（`AudioHelper`） |
| `relay/rerank_handler.go` | Rerank 入口 handler（`RerankHelper`） |
| `relay/websocket.go` | Realtime 入口 handler（`WssHelper`） |
| `relay/common_handler/rerank.go` | 统一 Rerank 响应 handler（含 xinference 分支） |
| `relaykit/dto/audio.go` | `AudioRequest`/`AudioResponse`/`WhisperVerboseJSONResponse`/`Segment`（relaykit 模块） |
| `relaykit/dto/rerank.go` | `RerankRequest`/`RerankResponse` 等 |
| `relaykit/dto/realtime.go` | `RealtimeEvent`/`RealtimeSession`/`RealtimeUsage` 等 + 12 个事件类型常量 |
| `relay/channel/openai/audio.go` | `OpenaiTTSHandler`/`OpenaiSTTHandler`（含 TTS 时长计费） |
| `relay/channel/openai/relay_realtime.go` | `OpenaiRealtimeHandler`/`preConsumeUsage`（双向 WS 桥接） |
| `relay/channel/minimax/tts.go` | MiniMax TTS |
| `relay/channel/volcengine/tts.go` | VolcEngine WS 流式 TTS |
| `relay/channel/ali/rerank.go` | 阿里 rerank 转换与响应解析 |
| `relay/channel/xinference/dto.go` | 仅含 `XinRerankResponse` 类型 |
| `relay/channel/volcengine/protocols.go` | Volcengine 私有二进制 WS 协议（`MsgType`/`EventType`/`Message`/`ReceiveMessage`/`FullClientRequest`），仅被 `tts.go` 使用 |
| `relay/channel/jina/`（整目录） | Jina 渠道：adaptor.go + constant.go |
| `service/audio.go` | `parseAudio`（仅被 CountAudioTokenInput/Output 调用）、`DecodeBase64AudioData`（无调用者） |
| `common/audio.go` | `GetAudioDuration` + 8 种格式解析器（仅被 token_counter STT 分支与 openai/audio.go 调用） |

### 2. 常量删除

- `relay/constant/relay_mode.go`：`RelayModeAudioSpeech/Transcription/Translation`、`RelayModeRerank`、`RelayModeRealtime` 定义 + `Path2RelayMode` 对应分支（iota 断号，后续常量数值前移）
- `relaykit/types/relay_format.go`：`RelayFormatOpenAIAudio`、`RelayFormatOpenAIRealtime`、`RelayFormatRerank`
- `relaykit/types/endpoint_type.go`：`EndpointTypeJinaRerank`（Jina 渠道默认端点）
- `relaykit/types/error.go`：`ErrorTypeRerankError`（全仓库唯一引用）
- `constant/endpoint_type.go`、`common/endpoint_type.go`：Jina/Rerank 端点别名与映射
- `common/endpoint_defaults.go`：Jina rerank 默认 path `/v1/rerank`

### 3. 接口与适配器改造

- `relay/channel/adapter.go`：删除接口方法 `ConvertAudioRequest`、`ConvertRerankRequest`
- 全部 34 个适配器的 `ConvertAudioRequest` 实现（26 个 stub + openai/minimax/volcengine/cloudflare/advancedcustom/siliconflow/newapi 真实现/委托）
- 全部 30 个适配器的 `ConvertRerankRequest` 实现（19 个 stub + ali/cohere/jina/siliconflow/openai/moonshot/cloudflare/advancedcustom 真实现/透传 + baidu_v2/newapi/codex 等报错型）
- 适配器内协议分支清理：
  - `openai/adaptor.go`：audio 转换/DoRequest(multipart)/DoResponse、realtime URL(scheme 替换/Azure 分支/Sec-WebSocket-Protocol 头)、rerank DoResponse 分支
  - `advancedcustom/adaptor.go`：audio 表单分支、rerank 委托、realtime `DoWssRequest` 与 URL scheme
  - `cloudflare/adaptor.go` + `relay_cloudflare.go` + `dto.go`：`cfSTTHandler`、`CfAudioResponse`
  - `ali/adaptor.go`：rerank URL 与响应分发
  - `cohere/adaptor.go` + `relay-cohere.go`：rerank URL、`requestConvertRerank2Cohere`、`cohereRerankHandler`
  - `siliconflow/adaptor.go` + `relay-siliconflow.go`：rerank URL、`siliconflowRerankHandler`
  - `moonshot/adaptor.go`、`baidu_v2/adaptor.go`、`volcengine/adaptor.go`：rerank URL 分支（volcengine 另删 TTS 分支）
  - `jina/adaptor.go` 随渠道删除
- 渠道 DTO：`ali/dto.go`（AliRerank\*）、`cohere/dto.go`（CohereRerank\*）、`siliconflow/dto.go`（SFRerankResponse）；`volcengine/protocols.go` 整文件删除（见第 1 节）

### 4. 请求链路改造

- `controller/relay.go`：包级 `upgrader`（仅 realtime 用）、`RelayFormatOpenAIRealtime` 分支（WS Upgrade）、defer 错误处理 WssError case、分发 switch 中 3 个协议 case
- `relay/helper/valid_request.go`：`GetAndValidAudioRequest`、`GetAndValidateRerankRequest`、realtime case（空 BaseRequest）
- `relay/helper/common.go`：`WssString`/`WssObject`/`WssError`/`GetLocalRealtimeID`
- `relay/common/relay_info.go`：7 个 Ws 字段（`ClientWs`/`TargetWs`/`InputAudioFormat`/`OutputAudioFormat`/`RealtimeTools`/`IsFirstRequest`/`AudioUsage`）、`ToLogInfo()` 中 Realtime 调试输出、`GenRelayInfoWs`/`GenRelayInfoOpenAIAudio`/`GenRelayInfoRerank` 及 `GenRelayInfo` switch 分支、`RerankerInfo` 结构
- `relay/channel/api_request.go`：multipart 跳过头逻辑（audio 分支）、realtime 分支（跳过 Content-Type）、`DoWssRequest`（保留其他通用函数）
- `middleware/distributor.go`：audio 路径分支（relay_mode 设置+默认模型 tts-1/whisper-1）、multipart 排除逻辑、realtime 分支（从 query 提取模型名）
- `middleware/auth.go`：`Sec-WebSocket-Protocol` 中 `openai-insecure-api-key.` 注入分支（唯一 WS 认证入口，realtime 是唯一 WS 客户端入口）、空函数 `WssAuth`（无调用者死代码）
- `controller/channel-test.go`：rerank 分支（转换/请求构建）、模型名 "rerank" 自动探测、relayFormat 映射
- `relaykit/relayconvert/convmeta/format.go`：`GuessRelayFormatFromRequest` 中 audio/rerank case
- `relaykit/dto/channel_settings.go`：`advancedCustomEndpointPathJinaRerank` 与 `EndpointTypeJinaRerank` 映射

### 5. 计费清理（专属逻辑）

- `service/token_counter.go`：
  - `EstimateRequestToken` 中 STT 分支（192-220，multipart 音频时长预扣估算）
  - `CountTokenRealtime`（303-355）、`CountAudioTokenInput`/`CountAudioTokenOutput`（377/389，仅被 realtime 计数使用）
  - realtime 格式跳过预估算的分支（189-191）
- `service/quota.go`：`PreWssConsumeQuota`（89-156）、`PostWssConsumeQuota`（158-259）
- `service/log_info_generate.go`：`GenerateWssOtherInfo`（248-258）
- `setting/ratio_setting/model_ratio.go`：`defaultAudioRatio`/`defaultAudioCompletionRatio` 中 tts/realtime 专属条目（`tts-1` 系列、`gpt-4o-mini-tts`、`gpt-4o-realtime-preview` 等；保留 `gpt-4o-audio-preview` 等 chat audio 模型条目）
- `setting/ratio_setting/cache_ratio.go`：`gpt-4o-realtime-preview` 条目
- `go.mod`：移除 6 个音频解析依赖（abema/go-mp4、go-audio/aiff、go-audio/wav、jfreymuth/oggvorbis、mewkiz/flac、tcolgate/mp3）及间接依赖，`go mod tidy` 处理

### 6. 模型列表清理（死条目）

- `relay/channel/openai/constant.go`：`gpt-4o-transcribe`、`gpt-4o-mini-transcribe`、`gpt-4o-mini-tts`、`whisper-1`、`tts-1` 系列、`gpt-realtime*` 系列、`gpt-realtime-whisper`、`gpt-realtime-translate`（保留 `gpt-4o-audio-preview`/`gpt-4o-mini-audio-preview` 等 chat audio 模型）
- `relay/channel/minimax/constants.go`：`speech-*` TTS 模型
- `relay/channel/gemini/constant.go`：`gemini-2.5-flash-preview-tts`（保留 audio-preview 类 chat 模型）
- `relay/channel/ali/constants.go`：`gte-rerank-v2`
- `relay/channel/cohere/constant.go`：4 个 rerank 模型
- `relay/channel/siliconflow/constant.go`：rerank 模型
- `relay/channel/xinference/constant.go`：rerank 模型
- `relay/channel/jina/constant.go`：随渠道删除

### 7. 前端清理（web/src）

- `features/channels/lib/advanced-custom.ts`：`ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS` 中 3 条 audio 路径、1 条 rerank（`/v1/rerank`）、1 条 realtime（`/v1/realtime`）选项
- `features/channels/lib/model-categories.ts`：`whisper-`、`tts-` 关键词分类
- `features/usage-logs/components/model-badge.tsx`：whisper/tts 徽章分类
- `features/channels/components/dialogs/channel-test-dialog.tsx`：`jina-rerank` 端点选项、`STREAM_INCOMPATIBLE_ENDPOINTS` 中该项
- `features/pricing/constants.ts`：`ENDPOINT_TYPES.JINA_RERANK` 及 label
- `features/models/constants.ts`：`ENDPOINT_TEMPLATES['jina-rerank']`
- `features/pricing/lib/mock-stats.ts`：`/embed|rerank/` 正则中的 rerank（audio profile 保留）
- `features/pricing/components/model-details-api.tsx`：`jina-rerank` 端点分支
- `features/channels/constants.ts` + `features/channels/lib/channel-utils.ts`：渠道类型 38 'Jina' 映射
- `features/usage-logs/components/dialogs/details-dialog.tsx`：`other?.ws` 判断（`other?.audio` 分支保留——chat audio tokens 日志仍需要）
- i18n：`web/src/i18n/static-keys.ts` 的 `'Rerank'` 键、locales 中 `"Rerank"`、`"Jina"` 键（经 i18n 脚本处理，不直接编辑 locale 文件）

### 8. 明确保留（勿动）

- `service/quota.go`：`PostAudioConsumeQuota`、`calculateAudioQuota`、`QuotaInfo`/`TokenDetails`（被 chat/responses 的 audio tokens 计费使用）
- `service/text_quota.go`：`AudioTokens` 拆分计费、`PostTextConsumeQuota`（通用）
- `service/tiered_settle.go`、`service/log_info_generate.go` 的 `GenerateAudioOtherInfo`
- `setting/ratio_setting/model_ratio.go` 的 audio 倍率设施（`GetAudioRatio` 等）
- `relaykit/dto/billing_usage.go`/`openai_response.go` 的 `Usage.AudioTokens` 字段及 relayconvert 中的搬运逻辑
- 前端 pricing/usage-logs 的 audio UI 与 i18n 键（audio 聊天模型计费展示）
- `gorilla/websocket` 依赖（xunfei 渠道 `relay-xunfei.go` 仍使用）
- 渠道类型常量（`ChannelTypeAli/MiniMax/VolcEngine` 等保留，仅删 Jina=38）
- 文档（用户决定本次不更新）

## 执行顺序

1. **relaykit 模块先行**（独立 go.mod）：`dto/{audio,rerank,realtime}.go`、`types/relay_format.go`、`types/endpoint_type.go`、`types/error.go`、`relayconvert/convmeta/format.go`、`dto/channel_settings.go` → `cd relaykit && GOWORK=off go build ./...`
2. **根模块常量与链路**：`relay_mode.go`、`controller/relay.go`、`valid_request.go`、`helper/common.go`、`relay_info.go`、`api_request.go`、`distributor.go`、`auth.go`、`channel-test.go`
3. **handler 与渠道**：删 4 个 handler 文件、`adapter.go` 接口、34+30 个适配器方法、Jina 渠道、各渠道协议分支与 DTO
4. **计费与模型列表**：`token_counter.go`、`quota.go`、`log_info_generate.go`、`service/audio.go`、`common/audio.go`、`model_ratio.go`、`cache_ratio.go`、各渠道 constant.go、go.mod
5. **前端**：10 个文件 + i18n（经脚本）
6. **验证**：根模块 `go build ./...` + `go vet` + 相关包测试；relaykit 独立构建；前端 `bun run typecheck`/lint/build/测试；codegraph sync 复查零残留

## 验证策略

- `go build ./...`、`go vet ./...`（根模块）
- `cd relaykit && GOWORK=off go build ./...`（relaykit 模块独立性，AGENTS.md 强制要求）
- `go test` 相关包：controller、relay/...、service、i18n
- 前端：`bun run typecheck`、lint（改动文件）、`bun run test`、`bun run build`
- `codegraph sync` + 查询 Audio/Rerank/Realtime/Jina/RealtimeUsage 等符号零残留
- grep 校验：`ConvertAudioRequest|ConvertRerankRequest|RelayModeAudio|RelayModeRerank|RelayModeRealtime|WssHelper|WssError|AudioHelper|RerankHelper|EndpointTypeJinaRerank|ChannelTypeJina` 零匹配

## 风险与注意

- **iota 断号**：`RelayModeResponses/Gemini/ResponsesCompact/AlphaSearch` 等常量数值前移。已确认前端不消费该数字，仅历史日志 `relay_mode` 列数字语义变化，可接受。
- **共享计费**：`PostAudioConsumeQuota`/`calculateAudioQuota`/audio 倍率被 chat/responses 的 audio tokens 模型复用，严禁删除。
- **relaykit 独立构建**：relaykit 下所有改动必须 `GOWORK=off` 验证。
- **gorilla/websocket 保留**：xunfei 渠道仍依赖，删除 realtime 后不得移除该依赖。
- **i18n 规范**：前端 locale 文件必须通过 i18n 脚本维护，禁止直接编辑。
- **计费安全不变量**：本次删除不触及计费转换路径（`common/quota_math.go` 等保持不动）。
