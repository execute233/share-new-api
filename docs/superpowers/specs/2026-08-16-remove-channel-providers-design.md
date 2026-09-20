> 历史方案：已被本次 main 集成方案取代。当前保留 main 全部功能；下文仅作历史记录，不作为实现要求。

# 删除海外/任务渠道供应商 — 设计文档

日期：2026-08-16
状态：已批准（用户确认设计方案）

## 背景与目标

项目是 AI API 网关（new-api），内置 40+ 渠道供应商适配器。用户希望精简：

1. 删除 15 个海外/小众 LLM 渠道
2. 删除 8 个任务/媒体类平台（视频/音频生成任务）
3. 任务链路（路由/处理器/前端任务日志视图）整体移除，`tasks` 表结构保留
4. 清理历史死代码（无适配器的 ChannelType 常量、废弃 billing case、注释死代码块）

目标：删除上述渠道与任务平台的完整链路（适配器 → 注册 → 类型常量 → 模型列表/倍率 → 渠道特判 → 前端 UI/i18n），保留 `tasks` 表结构与历史数据、共享基础设施。

## 决策记录

| 决策点 | 结论 |
|---|---|
| 删除方式 | 方案 1：全链路删除 + 常量 iota 断号（与上次删协议一致）；不做常量重编号压缩（会错位已存 DB 的 type 值） |
| 360(19)/LingYiWanWu(31) 死目录 | **保留**（用户未选择删除；仅含拼错的空常量文件，无适配器引用，无风险） |
| `tasks` 表 | 结构保留（历史数据不丢），任务读写/展示链路全部移除 |
| 任务前端日志视图 | 删除（usage-logs 的 task 分类、TaskLogs 列/过滤器/音频预览），日志分类保留 chat/image/audio |
| `Adapter.ConvertRerankRequest`/`ConvertAudioRequest` | 接口方法保留（通用 openai 路径 + 保留渠道的实现仍在），仅随渠道删除实现 |
| Gemini(24)/MiniMax(35)/Ali(17)/VolcEngine(45) 主渠道 | 保留（LLM 能力），其任务/视频子平台（Veo、hailuo、Ali 视频、DoubaoVideo）删除 |
| 文档（docs/learn、README×4、docs/openapi） | 本次不更新 |

## 删除范围

### 1. 删除的渠道适配器（15 个）

| 渠道 | ChannelType | 目录 | 说明 |
|---|---|---|---|
| PaLM | 11 | `relay/channel/palm/` | Google 老 API，Gemini 已覆盖 |
| VertexAI | 41 | `relay/channel/vertex/` | Google 云版 |
| AWS Bedrock | 33 | `relay/channel/aws/` | 含 rerank 能力实现 |
| Cohere | 34 | `relay/channel/cohere/` | 含 rerank 实现 |
| Mistral | 42 | `relay/channel/mistral/` | |
| Perplexity | 27 | `relay/channel/perplexity/` | |
| xAI | 48 | `relay/channel/xai/` | |
| Replicate | 56 | `relay/channel/replicate/` | |
| Ollama | 4 | `relay/channel/ollama/` | 本地推理；controller/channel.go 有 4 处模型列表特判需清理 |
| OpenRouter | 20 | `relay/channel/openrouter/` | 走 openai.Adaptor，删除 APITypeOpenRouter case |
| Cloudflare | 39 | `relay/channel/cloudflare/` | 含 cfSTTHandler/CfAudioResponse 音频能力 |
| Coze | 49 | `relay/channel/coze/` | |
| Dify | 37 | `relay/channel/dify/` | |
| Submodel | 53 | `relay/channel/submodel/` | |
| Xinference | 47 | `relay/channel/xinference/` | 走 openai.Adaptor，删除 APITypeXinference case |

### 2. 删除的任务平台（GetTaskAdaptor 全部注册）

| 平台 | ChannelType/Platform | 目录 |
|---|---|---|
| Suno | 36（ChannelTypeSunoAPI 一并删） | `relay/channel/task/suno/` |
| Kling | 50 | `relay/channel/task/kling/` |
| Jimeng | 51 | `relay/channel/task/jimeng/` |
| Vidu | 52 | `relay/channel/task/vidu/` |
| Sora | 55 | `relay/channel/task/sora/` |
| DoubaoVideo | 54 | `relay/channel/task/doubao/` |
| hailuo（MiniMax） | ChannelTypeMiniMax 任务 | `relay/channel/task/hailuo/` |
| Ali 视频任务 | ChannelTypeAli 任务 | `relay/channel/task/ali/` |
| Gemini（Veo）任务 | ChannelTypeGemini 任务 | `relay/channel/task/gemini/` |
| Vertex 任务 | ChannelTypeVertexAi 任务 | `relay/channel/task/vertex/` |

### 3. 任务链路移除（后端）

- `relay/relay_task.go`：任务处理器（轮询/回调/结算）
- `relay/relay_adaptor.go`：`GetTaskAdaptor` 整体删除（无任务平台后无调用者）；`GetAdaptor` 删除 15 个渠道 case
- 任务路由：`router/relay-router.go` 中 `/v1/tasks`、任务回调等路由
- `relay/constant/task_platform.go`（或所在文件）：`TaskPlatform*` 常量清理
- `service/` 任务结算/计费相关（task 预扣/结算/`attachQuotaSaturation` 的任务分支、task log 生成）
- `middleware/distributor.go` 任务分支、`controller/` 任务相关 handler（task 状态查询等）
- 渠道内任务入口：各保留渠道的 `TaskAdaptor`/任务 URL/`GetTaskPlatform` 引用
- **`model/task.go`（Task 结构体）保留**，`tasks` 表结构不变

### 4. 常量与映射清理

- `constant/channel.go`：删除 21 个渠道/任务 ChannelType 常量（15 渠道 + 6 任务独立常量：Kling=50/Jimeng=51/Vidu=52/Sora=55/DoubaoVideo=54/SunoAPI=36；hailuo/Ali/Gemini/Vertex 任务复用主渠道常量不删）与 10 个历史死常量（Midjourney=2/MidjourneyPlus=5/OpenAIMax=6/OhMyGPT=7/AILS=9/AIProxy=10/API2GPT=12/AIGC2D=13/AIProxyLibrary=21/FastGPT=22）；`ChannelBaseURLs` 对应条目置空（保留占位，iota 断号）；`ChannelTypeNames` 映射删除
- `constant/api_type.go`、`common/api_type.go`：删除对应 `APIType*` 常量与 `ChannelType2APIType` 分支
- `common/endpoint_type.go`、`constant/endpoint_type.go`、`common/endpoint_defaults.go`：OpenRouter/Sora/Xai/Cloudflare 等端点 case 清理
- `controller/model.go`：`init()` 遍历 `ChannelTypeDummy` 的逻辑自动适配（`Dummy` 常量保留），确认无硬编码渠道引用
- `relay/common/relay_info.go` 等：任务相关字段/`GenRelayInfo` 分支清理

### 5. 渠道特判清理

- `controller/channel.go`：Ollama 模型列表特判（4 处）、VertexAi 特判、Codex 之外的渠道特判
- `controller/channel-test.go`：`unsupportedTestChannelTypes` 中任务类型条目、Perplexity/Cohere 测试分支
- `controller/channel_upstream_update.go`：Ollama/Gemini/AdvancedCustom 之外的渠道 case 清理
- `controller/channel-billing.go`：废弃 case（AIProxy/API2GPT/AIGC2D 等）与注释死代码块（`//case common.ChannelTypeOpenAISB:` 等）清理
- `service/`、`relay/channel/` 内对删除渠道的引用（如 `codex_usage.go` 无；`channel.go` 的 `ChannelTypeAnthropic` 等保留）

### 6. 模型列表与倍率清理

- 删除渠道的模型常量：`ollama/`、`cohere/`（含 4 个 rerank 模型）、`mistral/`、`perplexity/`、`xai/`、`replicate/`、`dify/`、`coze/`、`cloudflare/`、`palm/`、`vertex/`、`aws/`、`submodel/`、`xinference/`、`openrouter/` 随渠道目录删除
- 任务模型：`sora-*`、`kling-*`、`jimeng-*`、`vidu-*`、`doubao-seed-*`（视频）、`suno-*`、`veo-*`、`hailuo-*`、`wan-*`（任务）等条目
- `setting/ratio_setting/model_ratio.go`、`cache_ratio.go`：删除渠道模型与任务模型的倍率条目
- `model/pricing_default.go`：vendor 规则中删除渠道的条目（aws、ollama、cohere 等）
- `relay/channel/openai/constant.go` 等保留渠道：确认无删除渠道专属模型混入

### 7. 前端清理（web/src）

- `features/channels/constants.ts`：渠道类型名/图标/颜色映射删除 21 个渠道/任务条目与 10 个历史死条目
- `features/channels/lib/channel-utils.ts`、`advanced-custom.ts`：渠道相关工具与端点路径清理
- `features/channels/components/dialogs/channel-test-dialog.tsx`：任务端点/删除渠道端点选项清理
- `features/pricing/*`（mock-stats、constants、model-details-api）：删除渠道模型条目与任务模型 profile
- `features/models/constants.ts`、`model-categories.ts`、`model-badge.tsx`：模型分类/徽章清理
- `features/usage-logs/`：task 分类删除——`task-logs-columns.tsx`、`task-logs-filter-bar.tsx`、`usage-logs-mobile-card.tsx` 的 TaskLogsCard、`dialogs/audio-preview-dialog.tsx`、`types.ts`/`section-registry.tsx` 的 task 分支；日志分类保留 chat/image/audio
- 其他任务相关页面/路由（如 `tasks` 页面）检查删除
- i18n：`static-keys.ts` 与 7 个 locale 文件中删除渠道名、任务相关键（经 i18n 脚本处理，不直接编辑 locale 文件）

### 8. 明确保留（勿动）

- OpenAI、Azure、Anthropic、Gemini、Baidu、BaiduV2、Zhipu、ZhipuV4、Ali、Xunfei、Tencent、MiniMax、Moonshot、DeepSeek、SiliconFlow、MokaAI、VolcEngine、AdvancedCustom、Codex、NewAPI、Sub2API、Custom 渠道
- `relay/channel/ai360/`、`relay/channel/lingyiwanwu/` 死目录（用户未选择删除）
- `model/task.go`（Task 结构体）与 `tasks` 表结构、历史数据
- `Adapter` 接口的 `ConvertRerankRequest`/`ConvertAudioRequest`（通用实现 + 保留渠道实现）
- relaykit 模块（其 dto 为通用格式，与具体渠道无耦合；渠道删除不触碰 relaykit）
- 聊天/嵌入/图像能力（chat、images、embeddings 协议链路）
- 文档（用户决定本次不更新）

## 执行顺序

1. **常量与映射**：`constant/channel.go`、`constant/api_type.go`、`common/api_type.go`、endpoint_type/endpoint_defaults
2. **适配器注册**：`relay/relay_adaptor.go`（GetAdaptor case + GetTaskAdaptor）、删除 15 个渠道目录、10 个任务目录
3. **任务链路**：`relay_task.go`、任务路由、service 任务结算、distributor 分支、controller 任务 handler
4. **渠道特判与 billing**：`controller/channel.go`、`channel-test.go`、`channel_upstream_update.go`、`channel-billing.go`
5. **模型与倍率**：model_ratio.go、cache_ratio.go、pricing_default.go、各保留渠道模型常量核对
6. **前端**：channels/pricing/models/usage-logs 文件 + i18n（经脚本）
7. **验证**：`go build ./...` + `go vet` + 相关包测试；relaykit `GOWORK=off go build ./...`；前端 typecheck/test/build；codegraph sync 复查零残留

## 验证策略

- 根模块：`go build ./...`、`go vet ./...`、`go test ./... -count=1`（预期仅 2 个预存不稳定测试失败：service channel_affinity 时间戳碰撞、relay/channel TestUpstreamGetBody 系列）
- relaykit：`cd relaykit && GOWORK=off go build ./...`（确认渠道删除不影响独立模块）
- 前端：`bun run typecheck`、`bun run test`、`bun run build`；lint 仅对改动文件确认无新增错误（仓库有预存 lint 错误，位于未改动文件）
- `gofmt -l` 干净（预存不洁文件 controller/misc.go 除外）
- `codegraph sync` + 查询 `ChannelTypeAws/Cohere/Mistral/Perplexity/Ollama/OpenRouter/Cloudflare/Coze/Dify/Submodel/Xinference/PaLM/VertexAi/Xai/Replicate/Suno/Kling/Jimeng/Vidu/Sora/DoubaoVideo/TaskPlatform/GetTaskAdaptor` 等符号零残留
- findstr 最终扫描：`GetTaskAdaptor|TaskPlatformSuno|ChannelTypeCohere|ChannelTypeMistral|...` 零匹配（排除 web/dist、node_modules、docs/）

## 风险与注意

- **iota 断号**：删除 21 个渠道/任务 + 10 个历史死 ChannelType 常量后，后续常量数值前移。已确认前端不消费该数字；DB 中已存渠道记录的 `type` 列数字语义不变（已存值不重映射），新渠道编号前移可接受。
- **`ChannelTypeDummy` 保留**：`controller/model.go` 的 `init()` 遍历 `1..ChannelTypeDummy` 枚举渠道类型，Dummy 必须保留且必须是最后一个常量。
- **任务表**：`tasks` 表结构、历史数据、`Task` 模型保留；只删读写/展示链路。`task` 类型的历史日志（logs 表）仍可通过 logs API 查询原始数据，但前端 task 分类视图删除。
- **渠道特判散落**：删除的渠道在 controller/service/middleware 中有分散特判（如 Ollama 4 处），必须逐个清理，否则留死分支或编译错误。
- **CountToken/计费**：删除的渠道专属计费（如任务预扣结算）随链路删除；通用计费（chat/image/embedding）不涉及。`attachQuotaSaturation` 的任务分支随任务链路删除，chat 分支保留。
- **前端 lint 预存错误**：位于未改动文件（model-actions.ts、multi-select.tsx、api-info.ts 等），本次不处理，仅确认改动文件无新增。
