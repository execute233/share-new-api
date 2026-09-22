# 删除 Audio / Rerank / Realtime 协议 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除 Audio（`/v1/audio/*`）、Rerank（`/v1/rerank`）、Realtime（`/v1/realtime`）三个协议的完整链路及 Jina 渠道（ChannelType=38），保留共享计费设施。

**Architecture:** 按协议分 4 个任务顺序执行，每个任务完成后仓库可独立编译。任务 1-3 分别删除一个协议的 handler/DTO/适配器分支/计费专属逻辑/模型列表/前端入口；任务 4 清理 go.mod 依赖并做全量验证与单次提交。relaykit 为独立 Go 模块，其内改动必须用 `GOWORK=off` 单独构建验证。

**Tech Stack:** Go 1.22+、Gin、GORM、React 19 + TypeScript + Bun、i18next

## Global Constraints

- relaykit 模块独立性：`relaykit/` 内所有改动必须 `cd relaykit && GOWORK=off go build ./...` 验证（AGENTS.md 强制）
- 单次提交：遵循 spec 决策（方案 A），4 个任务完成后在 Task 4 末尾一次性提交
- 保留（严禁删除）：`service/quota.go` 的 `PostAudioConsumeQuota`/`calculateAudioQuota`/`QuotaInfo`/`TokenDetails`、`service/text_quota.go` 的 AudioTokens 计费、`GenerateAudioOtherInfo`、audio 倍率设施（`GetAudioRatio` 等）、relaykit `Usage.AudioTokens` 字段、gorilla/websocket 依赖（xunfei 渠道 `relay/channel/xunfei/relay-xunfei.go` 使用）
- 保留：`gpt-4o-audio-preview`/`gpt-4o-mini-audio-preview` 等经 chat 使用的 audio 模型（模型列表与倍率条目）
- 前端 locale 文件（`web/src/i18n/locales/*.json`）禁止直接编辑，必须经 i18n 脚本处理
- 每任务完成后验证：根模块 `go build ./...` + 相关包 `go test`；改动 TS/TSX 后 `bun run typecheck` + lint
- 本次不更新文档（docs/learn、README、docs/openapi/relay.json）

---

### Task 1: 删除 Realtime 协议

**Files:**
- Delete: `relay/websocket.go`、`relay/channel/openai/relay_realtime.go`、`relaykit/dto/realtime.go`、`service/audio.go`
- Modify: `relaykit/types/relay_format.go`、`controller/relay.go`、`relay/common/relay_info.go`、`relay/helper/common.go`、`relay/helper/valid_request.go`、`relay/channel/api_request.go`、`relay/channel/openai/adaptor.go`、`relay/channel/advancedcustom/adaptor.go`、`middleware/distributor.go`、`middleware/auth.go`、`relay/constant/relay_mode.go`、`service/token_counter.go`、`service/quota.go`、`service/log_info_generate.go`、`setting/ratio_setting/model_ratio.go`、`setting/ratio_setting/cache_ratio.go`、`relay/channel/openai/constant.go`、`web/src/features/channels/lib/advanced-custom.ts`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

**Interfaces:**
- Consumes: 无（本任务先行）
- Produces: `GenRelayInfo` 签名去掉 `ws *websocket.Conn` 参数（Task 2/3 的 `controller/channel-test.go` 调用点已同步更新为无参形式）；`RelayModeRealtime`、`RelayFormatOpenAIRealtime` 常量不存在

- [ ] **Step 1: relaykit 清理**

1. 删除 `relaykit/dto/realtime.go`（含 `RealtimeEvent`/`RealtimeSession`/`RealtimeItem`/`RealtimeContent`/`RealtimeResponse`/`RealtimeUsage`/`RealTimeTool`/`InputAudioTranscription` 及 12 个事件类型常量）。
2. `relaykit/types/relay_format.go` 删除第 14 行 `RelayFormatOpenAIRealtime = "openai_realtime"`。
3. 验证：`cd relaykit && GOWORK=off go build ./...` 必须通过（relaykit 内无 realtime 引用）。

- [ ] **Step 2: 删除根模块整文件**

删除：`relay/websocket.go`（`WssHelper`）、`relay/channel/openai/relay_realtime.go`（`OpenaiRealtimeHandler`/`preConsumeUsage`）、`service/audio.go`（`parseAudio`/`DecodeBase64AudioData`，仅被 realtime token 计数使用）。

- [ ] **Step 3: 清理 `controller/relay.go`**

1. 删除包级变量 `upgrader`（258-263 行，`websocket.Upgrader{Subprotocols: []string{"realtime"}}`）。
2. 删除 `Relay()` 函数内 `ws *websocket.Conn` 变量声明（79 行）与 `RelayFormatOpenAIRealtime` 分支（82-90 行，upgrader.Upgrade + `helper.WssError`）。
3. defer 错误处理 switch 中删除 `case types.RelayFormatOpenAIRealtime: helper.WssError(c, ws, newAPIError.ToOpenAIError())`（97-98 行）。
4. `GenRelayInfo(c, relayFormat, request, ws)` 调用（123 行）改为 `GenRelayInfo(c, relayFormat, request)`。
5. 分发 switch 中删除 `case types.RelayFormatOpenAIRealtime: newAPIError = relay.WssHelper(c, relayInfo)`（220-222 行）。
6. 删除 import `"github.com/gorilla/websocket"`。
7. **同步修改** `controller/channel-test.go:231` 的调用：`relaycommon.GenRelayInfo(c, relayFormat, request, nil)` 改为 `relaycommon.GenRelayInfo(c, relayFormat, request)`（Task 1 修改了 GenRelayInfo 签名，此调用点必须同任务内更新，否则编译失败）。

- [ ] **Step 4: 清理 `relay/common/relay_info.go`**

1. `RelayInfo` 结构体删除 7 个字段（105-111 行）：`ClientWs`、`TargetWs`、`InputAudioFormat`、`OutputAudioFormat`、`RealtimeTools`、`IsFirstRequest`、`AudioUsage`。
2. 删除 `GenRelayInfoWs` 函数（351-359 行）。
3. `GenRelayInfo` 签名去掉 `ws *websocket.Conn` 参数（579 行），删除 `case types.RelayFormatOpenAIRealtime: info = GenRelayInfoWs(c, ws)`（589-590 行）。
4. `ToLogInfo()` 中删除 Realtime 字段的调试输出（约 280-284 行，涉及 `ClientWs`/`InputAudioFormat`/`OutputAudioFormat`/`RealtimeTools`/`IsFirstRequest`/`AudioUsage`）。
5. 删除 import `"github.com/gorilla/websocket"`（若不再使用）。

- [ ] **Step 5: 清理 `relay/helper/common.go` 与 `relay/helper/valid_request.go`**

1. `common.go` 删除 4 个函数：`WssString`（140-147）、`WssObject`（149-160）、`WssError`（162-172）、`GetLocalRealtimeID`（179-182）；删除 import `"github.com/gorilla/websocket"`；检查 `dto` import 是否仍被其他函数使用，若无则一并删除。
2. `valid_request.go` 删除 `GetAndValidateRequest` 中 `case types.RelayFormatOpenAIRealtime: request = &dto.BaseRequest{}`（52-53 行）。

- [ ] **Step 6: 清理 `relay/channel/api_request.go`、`openai/adaptor.go`、`advancedcustom/adaptor.go`**

1. `api_request.go`：`SetupApiRequestHeader` 中删除 `else if info.RelayMode == constant.RelayModeRealtime { // websocket }` 分支（54-55 行）；删除 `DoWssRequest` 函数（375-403 行）；删除 import `"github.com/gorilla/websocket"`。
2. `openai/adaptor.go` 删除 5 处 realtime 分支：
   - `GetRequestURL` 中 http→ws/https→wss scheme 替换（约 106-116 行）
   - Azure 分支 `/openai/realtime?deployment=...` URL（约 163-165 行）
   - `SetupRequestHeader` 中 `Sec-WebSocket-Protocol: realtime, openai-insecure-api-key.xxx` + `openai-beta: realtime=v1`（约 203-224 行，含 `legacyRealtimeBeta` 判断）
   - `DoRequest` 中 `channel.DoWssRequest`（约 628-629 行）
   - `DoResponse` 中 `OpenaiRealtimeHandler`（约 637-638 行）
3. `advancedcustom/adaptor.go` 删除 2 处：`DoRequest` 中 `DoWssRequest`（约 281-283 行）、`buildRouteURL` 中 realtime 的 scheme 替换（约 402-409 行）。

- [ ] **Step 7: 清理 `middleware/distributor.go` 与 `middleware/auth.go`**

1. `distributor.go` 删除 realtime 分支（354-357 行，`/v1/realtime` 从 query 提取 model）。
2. `auth.go` 删除 `TokenAuth` 中 WS 认证分支（355-368 行，`Sec-WebSocket-Protocol` 解析 `openai-insecure-api-key.`）；删除空函数 `WssAuth`（242-244 行）。

- [ ] **Step 8: 清理 `relay/constant/relay_mode.go`**

1. 删除第 50 行 `RelayModeRealtime`（及其后空行），iota 断号（`RelayModeGemini` 起数值前移 1）。
2. `Path2RelayMode` 删除 `else if strings.HasPrefix(path, "/v1/realtime")` 分支（91-92 行）。

- [ ] **Step 9: 清理计费相关**

1. `service/token_counter.go`：
   - 删除 `EstimateRequestToken` 中 `if info.RelayFormat == types.RelayFormatOpenAIRealtime { return 0, nil }`（189-191 行）
   - 删除 `CountTokenRealtime`（303-355 行）、`CountAudioTokenInput`（377-387 行）、`CountAudioTokenOutput`（389-399 行）
   - 检查 import `relaykit/dto` 是否仍被其他函数使用（`CountTokenInput` 等仍在），保留
2. `service/quota.go`：删除 `PreWssConsumeQuota`（89-156 行）、`PostWssConsumeQuota`（158-259 行）。**严禁删除** `calculateAudioQuota`、`QuotaInfo`、`TokenDetails`、`PostAudioConsumeQuota`、`PostConsumeQuota`。
3. `service/log_info_generate.go`：删除 `GenerateWssOtherInfo`（248-258 行）。**保留** `GenerateAudioOtherInfo`（260 行起）。
4. `setting/ratio_setting/model_ratio.go`：
   - `defaultModelRatio` 删除 `"gpt-4o-mini-tts": 0.3`（299 行）
   - `defaultAudioRatio` 删除 `"gpt-4o-realtime-preview"`（309）、`"gpt-4o-mini-realtime-preview"`（310）、`"gpt-4o-mini-tts": 25`（311）。**保留** `gpt-4o-audio-preview`（307）、`gpt-4o-mini-audio-preview`（308）
   - `defaultAudioCompletionRatio` 删除 `"gpt-4o-realtime"`（315）、`"gpt-4o-mini-realtime"`（316）、`"gpt-4o-mini-tts"`（317）、`tts-1` 系列（318-321）
5. `setting/ratio_setting/cache_ratio.go`：删除 `gpt-4o-realtime-preview` 条目（25-26 行）。
6. `relay/channel/openai/constant.go`：从模型列表删除 realtime 模型条目（`gpt-4o-realtime-preview*`、`gpt-realtime`、`gpt-realtime-mini`、`gpt-realtime-1.5`、`gpt-realtime-2*`、`gpt-realtime-whisper`、`gpt-realtime-translate`，约 52-62 行）。

- [ ] **Step 10: 清理前端**

1. `web/src/features/channels/lib/advanced-custom.ts`：删除 `ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS` 中 `{ value: '/v1/realtime', label: 'OpenAI Realtime' }`（151-153 行）。
2. `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`：第 498 行 `const hasAudioTokens = other?.ws || other?.audio` 改为 `const hasAudioTokens = other?.audio`。

- [ ] **Step 11: 验证 Task 1**

运行（工作目录 `E:\projs\go\share-new-api`）：
1. `go build ./...` — 必须通过
2. `cd relaykit && GOWORK=off go build ./...` — 必须通过
3. `go test ./controller/... ./service/... ./relay/... ./middleware/... -count=1` — 必须通过
4. `cd web && bun run typecheck` — 必须通过（若前端文件已改动）
5. grep 校验零残留：`RelayModeRealtime`、`RelayFormatOpenAIRealtime`、`WssHelper`、`WssError`、`WssObject`、`WssString`、`WssAuth`、`DoWssRequest`、`RealtimeUsage`、`CountTokenRealtime`、`PreWssConsumeQuota`、`PostWssConsumeQuota`、`GenerateWssOtherInfo`（应仅在测试或注释中无匹配）

---

### Task 2: 删除 Rerank 协议 + Jina 渠道

**Files:**
- Delete: `relay/rerank_handler.go`、`relay/common_handler/rerank.go`、`relaykit/dto/rerank.go`、`relay/channel/jina/`（整目录）、`relay/channel/ali/rerank.go`、`relay/channel/xinference/dto.go`
- Modify: `relaykit/types/relay_format.go`、`relaykit/types/endpoint_type.go`、`relaykit/types/error.go`、`relaykit/relayconvert/convmeta/format.go`、`relaykit/dto/channel_settings.go`、`constant/channel.go`、`constant/endpoint_type.go`、`common/api_type.go`、`common/endpoint_type.go`、`common/endpoint_defaults.go`、`relay/constant/relay_mode.go`、`controller/relay.go`、`controller/channel-test.go`、`relay/helper/valid_request.go`、`relay/common/relay_info.go`、`relay/channel/adapter.go` + 约 30 个渠道 adaptor、`relay/channel/ali/adaptor.go`、`relay/channel/ali/dto.go`、`relay/channel/ali/constants.go`、`relay/channel/cohere/adaptor.go`、`relay/channel/cohere/relay-cohere.go`、`relay/channel/cohere/dto.go`、`relay/channel/cohere/constant.go`、`relay/channel/siliconflow/adaptor.go`、`relay/channel/siliconflow/relay-siliconflow.go`、`relay/channel/siliconflow/dto.go`、`relay/channel/siliconflow/constant.go`、`relay/channel/moonshot/adaptor.go`、`relay/channel/baidu_v2/adaptor.go`、`relay/channel/volcengine/adaptor.go`、`relay/channel/openai/adaptor.go`、`relay/channel/advancedcustom/adaptor.go`、`relay/channel/cloudflare/adaptor.go`、`relay/channel/xinference/constant.go`、`web/src/features/channels/lib/advanced-custom.ts`、`web/src/features/channels/constants.ts`、`web/src/features/channels/lib/channel-utils.ts`、`web/src/features/channels/lib/model-categories.ts`、`web/src/features/channels/components/dialogs/channel-test-dialog.tsx`、`web/src/features/pricing/constants.ts`、`web/src/features/models/constants.ts`、`web/src/features/pricing/lib/mock-stats.ts`、`web/src/features/pricing/components/model-details-api.tsx`、`web/src/i18n/static-keys.ts`

**Interfaces:**
- Consumes: Task 1 的 `GenRelayInfo(c, relayFormat, request)`（无 ws 参数）
- Produces: `ChannelTypeJina`(38)、`EndpointTypeJinaRerank`、`RelayModeRerank`、`RelayFormatRerank`、`ErrorTypeRerankError`、`RerankerInfo`、`ConvertRerankRequest` 均不存在

- [ ] **Step 1: relaykit 清理**

1. 删除 `relaykit/dto/rerank.go`（`RerankRequest`/`RerankResponseResult`/`RerankDocument`/`RerankResponse`）。
2. `relaykit/types/relay_format.go` 删除第 15 行 `RelayFormatRerank = "rerank"`。
3. `relaykit/types/endpoint_type.go` 删除第 15 行 `EndpointTypeJinaRerank = "jina-rerank"`。
4. `relaykit/types/error.go` 删除第 34 行 `ErrorTypeRerankError ErrorType = "rerank_error"`。
5. `relaykit/relayconvert/convmeta/format.go` 删除 `case *dto.RerankRequest, dto.RerankRequest: return types.RelayFormatRerank, true`（22-23 行）。
6. `relaykit/dto/channel_settings.go`：删除 `advancedCustomEndpointPathJinaRerank = "/v1/rerank"`（143 行）与 `case advancedCustomEndpointPathJinaRerank: return types.EndpointTypeJinaRerank, true`（244-245 行）。
7. 验证：`cd relaykit && GOWORK=off go build ./...` 必须通过。

- [ ] **Step 2: 删除根模块整文件**

删除：`relay/rerank_handler.go`（`RerankHelper`）、`relay/common_handler/rerank.go`（`RerankHandler`，被 openai/adaptor.go 与 jina/adaptor.go 调用）、`relay/channel/jina/`（adaptor.go + constant.go）、`relay/channel/ali/rerank.go`（`ConvertRerankRequest` 转 AliRerankRequest + `RerankHandler`）、`relay/channel/xinference/dto.go`（仅 `XinRerankResponse`）。

- [ ] **Step 3: 清理 Jina 渠道类型链**

1. `constant/channel.go`：删除第 38 行 `ChannelTypeJina = 38` 与第 164 行名称映射 `ChannelTypeJina: "Jina"`。
2. `common/api_type.go`：删除第 40 行 `case constant.ChannelTypeJina:` 分支。
3. `common/endpoint_type.go`：删除 9-10 行 `case constant.ChannelTypeJina: endpointTypes = []constant.EndpointType{constant.EndpointTypeJinaRerank}`。
4. `common/endpoint_defaults.go`：删除第 26 行 `constant.EndpointTypeJinaRerank: {Path: "/v1/rerank", Method: "POST"}`。
5. `constant/endpoint_type.go`：删除第 16 行 `EndpointTypeJinaRerank = types.EndpointTypeJinaRerank`。

- [ ] **Step 4: 清理常量与请求链路**

1. `relay/constant/relay_mode.go`：删除第 46 行 `RelayModeRerank`；`Path2RelayMode` 删除 `else if strings.HasPrefix(path, "/v1/rerank")` 分支（89-90 行）。
2. `controller/relay.go`：`relayHandler` 删除 `case relayconstant.RelayModeRerank: err = relay.RerankHelper(c, info)`（47-48 行）。
3. `relay/helper/valid_request.go`：删除 `case types.RelayFormatRerank:`（48-49 行）与 `GetAndValidateRerankRequest` 函数（82-97 行）。
4. `relay/common/relay_info.go`：删除 `RerankerInfo` 结构（43-46 行）、`GenRelayInfoRerank`（372-381 行）、`GenRelayInfo` 中 `case types.RelayFormatRerank:` 分支（593-598 行）。
5. `controller/channel-test.go`：
   - 删除 `case constant.EndpointTypeJinaRerank: relayFormat = types.RelayFormatRerank`（194-195 行）与 219 行的 `/v1/rerank` 路径探测分支
   - 删除 `buildTestRequest` 中 rerank 分支（约 712-719 行，`case constant.EndpointTypeJinaRerank:` 构建 `RerankRequest`）
   - 删除模型名 "rerank" 自动探测分支（约 122-123、777-778 行）

- [ ] **Step 5: 删除 Adaptor 接口方法及全部实现**

1. `relay/channel/adapter.go`：删除第 22 行 `ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error)`。
2. 删除全部渠道 adaptor 的 `ConvertRerankRequest` 方法。完整清单（每处删除整个方法）：
   - 真实现/透传：`ali/adaptor.go`（224 行转调，连同 rerank.go 已删）、`cohere/adaptor.go`（71 行转调 `requestConvertRerank2Cohere`）、`siliconflow/adaptor.go`（105 行透传）、`openai/adaptor.go`（369 行透传）、`moonshot/adaptor.go`（110 行透传）、`cloudflare/adaptor.go`（75 行透传）、`advancedcustom/adaptor.go`（184-187 行委托）
   - stub（`return nil, nil`）：`baidu/adaptor.go:131`、`aws/adaptor.go:138`、`zhipu_4v/adaptor.go:96`、`zhipu/adaptor.go:69`、`gemini/adaptor.go:193`、`mokaai/adaptor.go:80`、`xunfei/adaptor.go:61`、`dify/adaptor.go:87`、`mistral/adaptor.go:60`、`deepseek/adaptor.go:154`、`xai/adaptor.go:94`、`minimax/adaptor.go:107`、`claude/adaptor.go:106`、`vertex/adaptor.go:321`、`perplexity/adaptor.go:69`、`tencent/adaptor.go:85`、`palm/adaptor.go:61`、`ollama/adaptor.go:77`、`volcengine/adaptor.go:320`
   - stub（`return nil, error`）：`baidu_v2/adaptor.go:100`、`newapi/adaptor.go:92`、`codex/adaptor.go:47`、`coze/adaptor.go:61`、`jimeng/adaptor.go:90`、`replicate/adaptor.go:509`、`submodel/adaptor.go:56`
   - `jina/adaptor.go` 已随渠道删除，不单独处理

- [ ] **Step 6: 清理渠道内 rerank 分支与 DTO**

1. `ali/adaptor.go`：删除 `GetRequestURL` 中 rerank URL 分支（约 108 行，`/api/v1/services/rerank/text-rerank/text-rerank`）与 `DoResponse` 中 rerank 分发（约 261 行）。
2. `ali/dto.go`：删除 `AliRerankParameters`/`AliRerankInput`/`AliRerankRequest`/`AliRerankResponse`（约 213-231 行）。
3. `ali/constants.go`：删除 `gte-rerank-v2` 模型条目（11 行）。
4. `cohere/adaptor.go`：删除 `GetRequestURL` rerank 分支（45 行）与 `DoResponse` 中 `cohereRerankHandler` 调用（81 行）；`relay-cohere.go` 删除 `requestConvertRerank2Cohere`（57-66 行）与 `cohereRerankHandler`（219-253 行）。
5. `cohere/dto.go`：删除 `CohereRerankRequest`/`CohereRerankResponseResult`（34-43 行）。
6. `cohere/constant.go`：删除 4 个 rerank 模型条目（9 行）。
7. `siliconflow/adaptor.go`：删除 `GetRequestURL` rerank 分支（69 行）与 `DoResponse` 中 `siliconflowRerankHandler` 调用（115 行）；`relay-siliconflow.go` 删除 `siliconflowRerankHandler`（16-45 行）。
8. `siliconflow/dto.go`：删除 `SFRerankResponse`（14-15 行）。
9. `siliconflow/constant.go`：删除 rerank 模型条目（48-49 行）。
10. `moonshot/adaptor.go`：删除 `GetRequestURL` 中 `/v1/rerank` 分支（64 行）。
11. `baidu_v2/adaptor.go`：删除 `GetRequestURL` 中 `/v2/rerank` 分支（56 行）。
12. `volcengine/adaptor.go`：删除 `GetRequestURL` 中 `/api/v3/rerank` 分支（272 行）。
13. `openai/adaptor.go`：删除 `DoResponse` 中 `common_handler.RerankHandler` 分支（约 651-652 行）。
14. `xinference/constant.go`：删除 rerank 模型条目（4-5 行）。

- [ ] **Step 7: 清理前端**

1. `web/src/features/channels/lib/advanced-custom.ts`：删除 `{ value: '/v1/rerank', label: 'OpenAI Rerank' }`（147-149 行）。
2. `web/src/features/channels/constants.ts`：删除第 61 行 `38: 'Jina'`。
3. `web/src/features/channels/lib/channel-utils.ts`：删除第 110 行 `38: 'Jina', // Jina`。
4. `web/src/features/channels/lib/model-categories.ts`：删除第 121 行 `{ name: 'Jina', keywords: ['jinaai', 'jina-'] },`。
5. `web/src/features/channels/components/dialogs/channel-test-dialog.tsx`：删除第 194 行 `{ value: 'jina-rerank', label: 'Jina Rerank (/v1/rerank)' }` 与第 209 行 `'jina-rerank',`（STREAM_INCOMPATIBLE_ENDPOINTS 中）。
6. `web/src/features/pricing/constants.ts`：删除第 74 行 `JINA_RERANK: 'jina-rerank',` 与第 93 行 `[ENDPOINT_TYPES.JINA_RERANK]: 'Rerank',`。
7. `web/src/features/models/constants.ts`：删除第 167 行 `'jina-rerank': { path: '/rerank', method: 'POST' },`。
8. `web/src/features/pricing/lib/mock-stats.ts`：第 231 行 `if (/embed|rerank/.test(n)) return 'embedding'` 改为 `if (/embed/.test(n)) return 'embedding'`。
9. `web/src/features/pricing/components/model-details-api.tsx`：第 433 行删除 `|| endpointType === 'jina-rerank'`。
10. `web/src/i18n/static-keys.ts`：删除第 67 行 `'Rerank',`。
11. i18n locales：运行 `cd web && bun run i18n:sync` 观察是否自动移除 `"Rerank"`/`"Jina"` 键；若未移除，参照 `scripts/remove-oauth-keys.mjs` 模式写一次性脚本删除 locales 中的 `Rerank`、`Jina`、`OpenAI Rerank` 键（7 种语言），执行后删除脚本。

- [ ] **Step 8: 验证 Task 2**

1. `go build ./...`、`cd relaykit && GOWORK=off go build ./...`、`go test ./controller/... ./relay/... -count=1` — 必须通过
2. `cd web && bun run typecheck && bun run lint` — 必须通过
3. grep 零残留：`ConvertRerankRequest`、`RelayModeRerank`、`RelayFormatRerank`、`RerankerInfo`、`RerankHelper`、`ErrorTypeRerankError`、`EndpointTypeJinaRerank`、`ChannelTypeJina`、`jina-rerank`

---

### Task 3: 删除 Audio 协议

**Files:**
- Delete: `relay/audio_handler.go`、`relaykit/dto/audio.go`、`relay/channel/openai/audio.go`、`relay/channel/minimax/tts.go`、`relay/channel/volcengine/tts.go`、`relay/channel/volcengine/protocols.go`、`common/audio.go`
- Modify: `relaykit/types/relay_format.go`、`relaykit/relayconvert/convmeta/format.go`、`relay/constant/relay_mode.go`、`controller/relay.go`、`relay/helper/valid_request.go`、`relay/common/relay_info.go`、`relay/channel/adapter.go` + 约 34 个渠道 adaptor、`relay/channel/openai/adaptor.go`、`relay/channel/minimax/adaptor.go`、`relay/channel/volcengine/adaptor.go`、`relay/channel/cloudflare/adaptor.go`、`relay/channel/cloudflare/relay_cloudflare.go`、`relay/channel/cloudflare/dto.go`、`relay/channel/advancedcustom/adaptor.go`、`relay/channel/siliconflow/adaptor.go`、`relay/channel/newapi/adaptor.go`、`relay/channel/api_request.go`、`middleware/distributor.go`、`service/token_counter.go`、`setting/ratio_setting/model_ratio.go`、`relay/channel/openai/constant.go`、`relay/channel/minimax/constants.go`、`relay/channel/gemini/constant.go`、`web/src/features/channels/lib/advanced-custom.ts`、`web/src/features/channels/lib/model-categories.ts`、`web/src/features/usage-logs/components/model-badge.tsx`

**Interfaces:**
- Consumes: Task 1/2 已删除 realtime/rerank 常量
- Produces: `RelayModeAudioSpeech/Transcription/Translation`、`RelayFormatOpenAIAudio`、`ConvertAudioRequest`、`GetAndValidAudioRequest`、`AudioHelper` 均不存在；`common.GetAudioDuration` 不存在

- [ ] **Step 1: relaykit 清理**

1. 删除 `relaykit/dto/audio.go`（`AudioRequest`/`AudioResponse`/`WhisperVerboseJSONResponse`/`Segment`）。
2. `relaykit/types/relay_format.go` 删除第 12 行 `RelayFormatOpenAIAudio = "openai_audio"`。
3. `relaykit/relayconvert/convmeta/format.go` 删除 `case *dto.AudioRequest, dto.AudioRequest: return types.RelayFormatOpenAIAudio, true`（26-27 行）。
4. 验证：`cd relaykit && GOWORK=off go build ./...` 必须通过。

- [ ] **Step 2: 删除根模块整文件**

删除：`relay/audio_handler.go`（`AudioHelper`）、`relay/channel/openai/audio.go`（`OpenaiTTSHandler`/`OpenaiSTTHandler`，含 TTS 时长计费）、`relay/channel/minimax/tts.go`（`MiniMaxTTSRequest` 等 + `handleTTSResponse`）、`relay/channel/volcengine/tts.go`（`VolcengineTTSRequest` 等 + `handleTTSResponse`/`handleTTSWebSocketResponse`）、`relay/channel/volcengine/protocols.go`（`MsgType`/`EventType`/`Message`/`ReceiveMessage`/`FullClientRequest`，仅被 tts.go 使用）、`common/audio.go`（`GetAudioDuration` + 8 种格式解析器）。

- [ ] **Step 3: 清理常量与请求链路**

1. `relay/constant/relay_mode.go`：删除 35-37 行三个常量（`RelayModeAudioSpeech`/`RelayModeAudioTranscription`/`RelayModeAudioTranslation`）；`Path2RelayMode` 删除 83-88 行三个 `/v1/audio/*` 分支。
2. `controller/relay.go`：`relayHandler` 删除 audio 三个 case（41-46 行，`case relayconstant.RelayModeAudioSpeech: fallthrough ...`）。
3. `relay/helper/valid_request.go`：删除 `case types.RelayFormatOpenAIAudio:`（50-51 行）与 `GetAndValidAudioRequest` 函数（60-80 行）。
4. `relay/common/relay_info.go`：删除 `GenRelayInfoOpenAIAudio`（383-387 行）与 `GenRelayInfo` 中 `case types.RelayFormatOpenAIAudio:`（585-586 行）。
5. `relay/channel/api_request.go`：`SetupApiRequestHeader` 删除 `if info.RelayMode == constant.RelayModeAudioTranscription || info.RelayMode == constant.RelayModeAudioTranslation { // multipart/form-data }` 分支（52-53 行），`else if` 改回直接 `req.Set(...)`（原 56-62 行的 `else` 块）。
6. `middleware/distributor.go`：
   - 第 347 行条件 `!strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") && !strings.Contains(...)` 恢复为 `!strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data")`
   - 删除 380-401 行 `/v1/audio` 分支（relay_mode 设置与 tts-1/whisper-1 默认模型）
7. `service/token_counter.go`：删除 `EstimateRequestToken` 中 STT 分支（192-220 行，multipart 音频时长预扣估算，含 `common.GetAudioDuration` 调用与 `common.QuotaRound` 换算）；检查 import `path/filepath`、`math`、`common` 是否仍被其他代码使用。

- [ ] **Step 4: 删除 Adaptor 接口方法及全部实现**

1. `relay/channel/adapter.go`：删除第 24 行 `ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error)`。
2. 删除全部渠道 adaptor 的 `ConvertAudioRequest` 方法：
   - 真实现：`openai/adaptor.go`（377-439 行，speech JSON body + transcription/translation multipart 构造）、`minimax/adaptor.go`（35 行，转 tts.go）、`volcengine/adaptor.go`（49 行，转 tts.go）、`cloudflare/adaptor.go`（83-98 行）
   - 委托：`advancedcustom/adaptor.go`（162-170 行委托 openaiAdaptor）、`siliconflow/adaptor.go`（34-36 行委托 openai）、`newapi/adaptor.go`（96-98 行报错）
   - stub（27 个，其中 jina 渠道已于 Task 2 删除、跳过）：`ali/adaptor.go:232`、`aws/adaptor.go:78`、`cohere/adaptor.go:31`、`dify/adaptor.go:38`、`coze/adaptor.go:28`、`deepseek/adaptor.go:46`、`baidu_v2/adaptor.go:33`、`claude/adaptor.go:32`、`codex/adaptor.go:32`、`replicate/adaptor.go:517`、`baidu/adaptor.go:32`、`moonshot/adaptor.go:36`、`jimeng/adaptor.go:98`、`perplexity/adaptor.go:33`、`zhipu_4v/adaptor.go:34`、`mistral/adaptor.go:30`、`mokaai/adaptor.go:32`、`jina/adaptor.go:33`（跳过）、`palm/adaptor.go:31`、`zhipu/adaptor.go:31`、`gemini/adaptor.go:58`、`xunfei/adaptor.go:31`、`vertex/adaptor.go:106`、`ollama/adaptor.go:39`、`submodel/adaptor.go:28`、`tencent/adaptor.go:39`、`xai/adaptor.go:35`

- [ ] **Step 5: 清理渠道内 audio 分支**

1. `openai/adaptor.go`：删除 `DoRequest` 中 multipart 表单请求分支（约 624-627 行，`channel.DoFormRequest`）与 `DoResponse` 中 audio 分发（约 639-643 行）。
2. `minimax/adaptor.go`：删除 TTS 相关（36 行 `RelayModeAudioSpeech` URL 路由、124 行响应分发）。
3. `volcengine/adaptor.go`：删除 TTS 分支（50 行 `RelayModeAudioSpeech`、276-280 行 GetRequestURL、333 行 DoRequest、356 行 DoResponse）；删除 import `"github.com/gorilla/websocket"`。
4. `cloudflare/adaptor.go`：删除 121-123 行 STT 响应处理（`RelayModeAudioTranscription/Translation`）；`relay_cloudflare.go` 删除 `cfSTTHandler`（122-148 行）；`dto.go` 删除 `CfAudioResponse`（15 行）。
5. `advancedcustom/adaptor.go`：删除 276-277 行 audio 表单请求分支。

- [ ] **Step 6: 清理倍率与模型列表**

1. `setting/ratio_setting/model_ratio.go`：删除 `defaultModelRatio` 中 `"gpt-4o-mini-tts": 0.3`（299 行，若 Task 1 未删）。
2. `relay/channel/openai/constant.go`：删除 `gpt-4o-transcribe`、`gpt-4o-mini-transcribe`、`gpt-4o-mini-tts`、`whisper-1`、`tts-1`、`tts-1-hd`、`tts-1-1106`、`tts-1-hd-1106` 模型条目（13-17、51-62 行）。**保留** `gpt-4o-audio-preview`、`gpt-4o-mini-audio-preview`。
3. `relay/channel/minimax/constants.go`：删除 `speech-*` TTS 模型条目（13-18 行）。
4. `relay/channel/gemini/constant.go`：删除 `gemini-2.5-flash-preview-tts` 条目（13 行）。**保留** `gemini-2.5-flash-preview-*audio*` 类 chat 模型。

- [ ] **Step 7: 清理前端**

1. `web/src/features/channels/lib/advanced-custom.ts`：删除 3 条 audio 选项（135-145 行，`/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations`）。
2. `web/src/features/channels/lib/model-categories.ts`：删除第 40 行 `'whisper-',` 关键词与第 53 行 `tts-` 正则分支中的 `tts-` 部分（保留 `o(?:1|3|4)` 部分）。
3. `web/src/features/usage-logs/components/model-badge.tsx`：删除第 54-55 行 `'whisper'`、`'tts-'` 关键词。

- [ ] **Step 8: 验证 Task 3**

1. `go build ./...`、`cd relaykit && GOWORK=off go build ./...`、`go test ./controller/... ./service/... ./relay/... -count=1` — 必须通过
2. `cd web && bun run typecheck` — 必须通过
3. grep 零残留：`ConvertAudioRequest`、`RelayModeAudio`、`RelayFormatOpenAIAudio`、`AudioHelper`、`GetAndValidAudioRequest`、`GetAudioDuration`、`OpenaiTTSHandler`、`OpenaiSTTHandler`、`Wss`（在 relay/ service/ 下应无匹配；xunfei 的 `websocket.Dialer` 除外）

---

### Task 4: 依赖清理 + 全量验证 + 提交

**Files:**
- Modify: `go.mod`、`go.sum`

**Interfaces:**
- Consumes: 全部 3 个协议已删除

- [ ] **Step 1: 清理 go.mod 音频解析依赖**

运行 `go mod tidy`。验证以下 6 个依赖已从 `go.mod` 移除（含间接依赖）：`github.com/abema/go-mp4`、`github.com/go-audio/aiff`、`github.com/go-audio/wav`、`github.com/jfreymuth/oggvorbis`、`github.com/mewkiz/flac`、`github.com/tcolgate/mp3`（以及 `github.com/go-audio/audio`、`github.com/go-audio/riff`、`github.com/mewkiz/pkg` 等 indirect）。
若 `go mod tidy` 未移除（被其他代码引用），grep `go-mp4|go-audio|oggvorbis|mewkiz|tcolgate` 定位引用并确认。

- [ ] **Step 2: 全量验证**

1. 后端：`go build ./...` + `go vet ./...` + `go test ./... -count=1`（全仓库测试）
2. relaykit：`cd relaykit && GOWORK=off go build ./...`
3. 前端：`cd web && bun run typecheck && bun run lint && bun run test && bun run build`
4. codegraph 复查：`codegraph sync` 后 `codegraph query` 查询 `RealtimeUsage`、`RerankRequest`、`AudioRequest`、`ChannelTypeJina`、`ConvertAudioRequest`、`ConvertRerankRequest` — 全部 No results
5. grep 最终校验（全仓库，排除 `web/dist`、`node_modules`、`docs/`）：
   ```
   ConvertAudioRequest|ConvertRerankRequest|RelayModeAudio|RelayModeRerank|RelayModeRealtime|RelayFormatOpenAIAudio|RelayFormatOpenAIRealtime|RelayFormatRerank|WssHelper|WssError|WssObject|WssString|AudioHelper|RerankHelper|EndpointTypeJinaRerank|ChannelTypeJina|RealtimeUsage|RerankRequest|AudioRequest
   ```
   零匹配（README/docs 按用户决策保留，故 docs/ 排除）

- [ ] **Step 3: 单次提交**

```bash
git add -A
git status   # 确认无意外文件（排除 .codegraph/ 已忽略或手动 git add 指定文件）
git commit -m "refactor(relay): remove Audio, Rerank, Realtime protocols and Jina channel

- Delete /v1/audio/*, /v1/rerank, /v1/realtime endpoints, handlers,
  adaptor methods (ConvertAudioRequest/ConvertRerankRequest), DTOs and
  protocol constants; iota numbering of remaining RelayMode values shifts
- Remove Jina channel (type 38, rerank-only) and its endpoint type
- Drop realtime/tts/whisper/rerank model list entries and ratio defaults;
  keep chat-usable audio models (gpt-4o-audio-preview etc.) and shared
  audio-token billing infrastructure
- Remove audio-format parsing dependencies from go.mod
- Clean up frontend channel/pricing UI entries and i18n keys"
```

注意：`.codegraph/` 为未跟踪工具产物，若 `git add -A` 会纳入，应改用显式文件列表或确认 `.gitignore` 已覆盖。

---

## Self-Review 记录

**Spec 覆盖检查：**
- Spec 第 1 节整文件删除（16 个文件）→ Task 1 Step 2、Task 2 Step 2、Task 3 Step 2 全覆盖
- Spec 第 2 节常量删除 → Task 1 Step 1/8、Task 2 Step 1/3/4、Task 3 Step 1/3 全覆盖
- Spec 第 3 节接口与适配器 → Task 2 Step 5/6、Task 3 Step 4/5 全覆盖
- Spec 第 4 节请求链路 → Task 1 Step 3-7、Task 2 Step 4、Task 3 Step 3 全覆盖
- Spec 第 5 节计费清理 → Task 1 Step 9、Task 3 Step 3/6 全覆盖
- Spec 第 6 节模型列表 → Task 1 Step 9、Task 2 Step 6、Task 3 Step 6 全覆盖
- Spec 第 7 节前端 → Task 1 Step 10、Task 2 Step 7、Task 3 Step 7 全覆盖
- Spec 第 8 节保留项 → 各 Task 明确标注"严禁删除"清单
- Spec 验证策略 → Task 1/2/3 末尾验证 + Task 4 Step 2 全量验证

**已知偏差：** `relay/channel/jina/adaptor.go:33` 的 `ConvertAudioRequest` 位于 Task 3 Step 4 清单中，但 jina 渠道已在 Task 2 删除——Task 3 执行时跳过该文件（清单已标注）。`controller/channel-test.go` 中 audio 相关探测分支：实际代码中无 audio 探测（grep 确认仅 rerank），无需处理。
