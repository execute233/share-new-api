# 删除海外/任务渠道供应商 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除 15 个海外/小众渠道、10 个任务平台、任务链路与历史死代码，只保留核心国产 + OpenAI 系渠道。

**Architecture:** 渠道适配器集中在 `relay/channel/`（每渠道一个目录），注册点在 `relay/relay_adaptor.go`，类型常量在 `constant/channel.go`/`common/api_type.go`。任务链路横跨 `relay/relay_task.go`、`service/task_*.go`、`controller/task.go`/`relay.go`、`router/*.go`、`middleware/*_adapter.go`。删除顺序：常量 → 渠道 → 任务 → 特判 → 模型倍率 → 前端 → 验证。

**Tech Stack:** Go 1.25 / Gin / GORM；前端 React 19 / TypeScript / Bun。

**Spec:** `docs/superpowers/specs/2026-08-16-remove-channel-providers-design.md`

## Global Constraints

- **iota 断号**：`ChannelType*`/`APIType*` 常量删除后不重编号，**保留常量的数值保持原样（跳号）**——spec 明确"不做常量重编号压缩（会错位已存 DB 的 type 值）"。`ChannelBaseURLs` 数组保持 61 项占位。`ChannelTypeDummy` 必须保留且仍是最后一个常量（`controller/model.go` 的 `init()` 遍历 `1..ChannelTypeDummy`）。
- **保留渠道**：OpenAI、Azure、Anthropic、Gemini、Baidu、BaiduV2、Zhipu、ZhipuV4、Ali、Xunfei、Tencent、MiniMax、Moonshot、DeepSeek、SiliconFlow、MokaAI、VolcEngine、AdvancedCustom、Codex、NewAPI、Sub2API、Custom。`relay/channel/ai360/`、`relay/channel/lingyiwanwu/` 死目录**不删**（用户未选）。
- **Midjourney 功能保留**：`relay/mjproxy_handler.go`、`RelayModeMidjourney*`、`/mj` 路由、`GetUserMidjourney`/`GetAllMidjourney`、`controller/midjourney.go`、`service/midjourney.go`、`model/midjourney.go`、`TaskPlatformMidjourney` 全部保留。只删 `ChannelTypeMidjourney`/`ChannelTypeMidjourneyPlus` 常量及其引用。
- **`tasks` 表与 Task 模型保留**：`model/task.go` 的 `Task` 结构体及 `InitTask`/`GetByTaskId` 等方法保留（`middleware/distributor.go:409` 用 `GetByTaskId` 解析模型名）。
- **relaykit**：`types/relay_format.go` 的 `RelayFormatTask` 删除（仅 `relay/common/relay_info.go:562` 引用，relaykit 内无其他使用）；其余 relaykit 代码不动。任何 relaykit 改动必须 `cd relaykit && GOWORK=off go build ./...` 验证。
- **前端 i18n**：locale 文件禁止直接编辑，用一次性脚本处理（沿用删除 Rerank/Jina 键的脚本模式）。
- **提交策略**：全部 Task 完成并全量验证通过后，单次提交（用户指令"不 commit 直到计划完成"）。计划文档本身用 `git add -f`（`docs/superpowers/plans/` 被 gitignore）。
- 测试：仅删除随被删代码的测试文件（`service/task_billing_test.go`、`service/task_polling_test.go` 等），保留与保留代码相关的测试。全量 `go test ./...` 预期只有 2 个预存不稳定测试失败（service channel_affinity、relay/channel TestUpstreamGetBody）。

---

### Task 1: 渠道类型常量与映射删除

**Files:**
- Modify: `constant/channel.go`
- Modify: `constant/api_type.go`
- Modify: `common/api_type.go`
- Modify: `common/endpoint_type.go`、`constant/endpoint_type.go`、`common/endpoint_defaults.go`
- Modify: `controller/channel-test.go`（编译修复）
- Modify: `controller/model.go`（如编译报错）

**Interfaces:**
- Consumes: 无（第一个任务）
- Produces: 删除后的 `constant.ChannelType*` 常量集合（后续任务引用）

- [ ] **Step 1: 删除渠道类型常量**

编辑 `constant/channel.go`：
- 删除渠道常量：`ChannelTypePaLM(11)`、`ChannelTypeOpenRouter(20)`、`ChannelTypePerplexity(27)`、`ChannelTypeAws(33)`、`ChannelTypeCohere(34)`、`ChannelTypeSunoAPI(36)`、`ChannelTypeDify(37)`、`ChannelCloudflare(39)`、`ChannelTypeVertexAi(41)`、`ChannelTypeMistral(42)`、`ChannelTypeXinference(47)`、`ChannelTypeXai(48)`、`ChannelTypeCoze(49)`、`ChannelTypeKling(50)`、`ChannelTypeJimeng(51)`、`ChannelTypeVidu(52)`、`ChannelTypeSubmodel(53)`、`ChannelTypeDoubaoVideo(54)`、`ChannelTypeSora(55)`、`ChannelTypeReplicate(56)`
- 删除历史死常量：`ChannelTypeMidjourney(2)`、`ChannelTypeMidjourneyPlus(5)`、`ChannelTypeOpenAIMax(6)`、`ChannelTypeOhMyGPT(7)`、`ChannelTypeAILS(9)`、`ChannelTypeAIProxy(10)`、`ChannelTypeAPI2GPT(12)`、`ChannelTypeAIGC2D(13)`、`ChannelTypeAIProxyLibrary(21)`、`ChannelTypeFastGPT(22)`
- **明确保留**（死目录/常量用户未选删除）：`ChannelTypeUnknown`、`ChannelTypeOpenAI`、`ChannelTypeAzure`、`ChannelTypeOllama(4)`、`ChannelTypeCustom`、`ChannelTypeAnthropic`、`ChannelTypeBaidu`、`ChannelTypeZhipu`、`ChannelTypeAli`、`ChannelTypeXunfei`、`ChannelType360(19)`、`ChannelTypeTencent`、`ChannelTypeGemini`、`ChannelTypeMoonshot`、`ChannelTypeZhipu_v4`、`ChannelTypeLingYiWanWu(31)`、`ChannelTypeMiniMax`、`ChannelTypeSiliconFlow`、`ChannelTypeMokaAI`、`ChannelTypeVolcEngine`、`ChannelTypeBaiduV2`、`ChannelTypeCodex`、`ChannelTypeAdvancedCustom`、`ChannelTypeSub2API`、`ChannelTypeNewAPI`、`ChannelTypeDummy`
- 先执行 `findstr /s /n "ChannelTypeLingYiWanWu\|ChannelType360\|ChannelTypeOllama" *.go` 确认保留常量引用点
- `ChannelBaseURLs`：**保留数组全部 61 项（索引与原始 ChannelType 数值一一对应），被删位置置空字符串 "" 占位，保留位置维持原 URL 不变**；`ChannelTypeNames` map 删除对应条目（保留项的 key 数值不变）

- [ ] **Step 2: 删除 APIType 常量与映射**

- `constant/api_type.go`：删除 `APITypePaLM`、`APITypeOpenRouter`、`APITypePerplexity`、`APITypeAws`、`APITypeCohere`、`APITypeDify`、`APITypeCloudflare`、`APITypeVertexAi`、`APITypeMistral`、`APITypeXinference`、`APITypeXai`、`APITypeCoze`、`APITypeSubmodel`、`APITypeReplicate` 及 `APITypeAIProxyLibrary` 等死常量。**`APITypeJimeng` 保留到 Task 3**（仍被 `GetAdaptor` 的 jimeng case 引用，随任务链路删除）
- `common/api_type.go`：`ChannelType2APIType` switch 删除对应 case（含 `case constant.ChannelTypeJimeng`——常量已在 Step 1 删除，映射必须同步删）
- `common/endpoint_type.go`/`constant/endpoint_type.go`/`common/endpoint_defaults.go`：删除 OpenRouter/Sora/Xai/VertexAi/Aws 等被删渠道的 case 与端点映射

- [ ] **Step 3: 编译修复（迭代）**

Run: `go build ./...`
- `controller/channel-test.go:77-87`：`unsupportedTestChannelTypes` 中删除 `ChannelTypeMidjourney`/`ChannelTypeMidjourneyPlus`/`ChannelTypeSunoAPI`/`ChannelTypeKling`/`ChannelTypeJimeng`/`ChannelTypeDoubaoVideo`/`ChannelTypeVidu`（此列表其他保留）
- 其余编译错误逐个修复（引用被删常量的位置直接删分支）
- 同时清理被删渠道的硬编码引用（如 `channel-upstream_update.go`、`channel-billing.go` 中的 case——本 Task 只保证编译通过，详细清理在 Task 4）

- [ ] **Step 4: 验证**

Run: `go build ./... && go vet ./...`
Expected: 成功

- [ ] **Step 5: 提交（不执行——全局单次提交在 Task 7）**

---

### Task 2: 删除 15 个渠道适配器

**Files:**
- Modify: `relay/relay_adaptor.go`
- Delete: `relay/channel/palm/`、`relay/channel/vertex/`、`relay/channel/aws/`、`relay/channel/cohere/`、`relay/channel/mistral/`、`relay/channel/perplexity/`、`relay/channel/xai/`、`relay/channel/replicate/`、`relay/channel/ollama/`、`relay/channel/openrouter/`、`relay/channel/cloudflare/`、`relay/channel/coze/`、`relay/channel/dify/`、`relay/channel/submodel/`、`relay/channel/xinference/`

**Interfaces:**
- Consumes: Task 1 的常量删除结果
- Produces: `GetAdaptor` 只保留保留渠道的 case

- [ ] **Step 1: 删除 GetAdaptor case**

编辑 `relay/relay_adaptor.go` 的 `GetAdaptor`：
- 删除 15 个渠道的 case（Ollama/OpenRouter/Xinference 的 openai 复用 case 一并删）：
  `APITypePaLM`、`APITypePerplexity`、`APITypeAws`、`APITypeCohere`、`APITypeDify`、`APITypeCloudflare`、`APITypeVertexAi`、`APITypeMistral`、`APITypeXinference`、`APITypeXai`、`APITypeCoze`、`APITypeSubmodel`、`APITypeReplicate`、`APITypeOpenRouter`、`APITypeOllama`
- 同时删除对应 import（`aws`、`cohere`、`mistral`、`perplexity`、`xai`、`replicate`、`ollama`、`openrouter`、`cloudflare`、`coze`、`dify`、`submodel`、`palm`、`vertex`、`xinference`）
- **APITypeJimeng 与 `&jimeng.Adaptor{}` case 保留到 Task 3**（jimeng 是任务渠道，随任务链路删除；提前删会让本 Task 与 Task 3 交叉）
- `controller/channel.go` 的 Ollama 特判（4 处）在 Task 4 清理；若本 Task 编译失败（Ollama case 删除导致），在本 Task 一并删除对应特判

- [ ] **Step 2: 删除渠道目录**

Run: `rmdir /s /q relay\channel\palm relay\channel\vertex relay\channel\aws relay\channel\cohere relay\channel\mistral relay\channel\perplexity relay\channel\xai relay\channel\replicate relay\channel\ollama relay\channel\openrouter relay\channel\cloudflare relay\channel\coze relay\channel\dify relay\channel\submodel relay\channel\xinference`

- [ ] **Step 3: 编译修复（迭代）**

Run: `go build ./...` 反复直到通过
- 处理引用被删渠道代码的位置（如 `relay/channel/adapter.go` 无渠道引用；`controller/model.go` 的模型列表来源、`service/` 中对渠道的引用）
- 注意 `relay/relay_task.go` 还引用任务渠道（Task 3 处理，若编译失败先注释或继续删）——为保持任务独立可先让 Task 2 只删 LLM 渠道，`task/` 目录留到 Task 3

- [ ] **Step 4: 验证**

Run: `go build ./... && go vet ./... && go test ./relay/channel/... ./controller/... -count=1`
Expected: 成功

- [ ] **Step 5: 提交（不执行）**

---

### Task 3: 任务链路整体移除

**Files:**
- Modify: `relay/relay_adaptor.go`（`GetTaskAdaptor` 删除、`APITypeJimeng` case 删除）
- Delete: `relay/relay_task.go`、`relay/channel/task/`（整目录，含 taskcommon）、`relay/channel/jimeng/`（Jimeng 渠道适配器，GetAdaptor 的 APITypeJimeng 引用）
- Delete: `dto/task.go`、`controller/task.go`
- Modify: `controller/relay.go`（RelayTaskFetch/RelayTask/respondTaskError/shouldRetryTaskRelay/RelayTaskSubmit 引用）
- Delete: `service/task.go`、`service/task_billing.go`、`service/task_polling.go` 及 `service/task_billing_test.go`、`service/task_polling_test.go`
- Modify: `router/main.go`、Delete: `router/video-router.go`
- Modify: `router/relay-router.go`（relaySunoRouter 块 160-168）、`router/api-router.go`（/api/task 路由 321-325）
- Delete: `middleware/kling_adapter.go`、`middleware/jimeng_adapter.go`、`controller/video_proxy.go`、`controller/video_proxy_gemini.go`
- Modify: `main.go`（GetTaskAdaptorFunc 工厂 137-138）、`controller/system_task_handlers.go`（RunTaskPollingOnce 相关 handler）
- Modify: `relaykit/types/relay_format.go`（`RelayFormatTask`）、`relay/common/relay_info.go`（:562 case）
- Modify: `constant/task.go`（删 Suno 部分，保留 `TaskPlatform` 类型与 `TaskPlatformMidjourney`）
- Keep: `model/task.go`（Task 结构体与方法保留）

**Interfaces:**
- Consumes: Task 1-2
- Produces: 无任务链路残留；`middleware.Distribute()` 不再有任务分支

- [ ] **Step 1: 删除 relay 层任务代码**

- 删除 `relay/relay_task.go`、`relay/channel/task/` 整目录、`relay/channel/jimeng/` 目录
- `relay/relay_adaptor.go`：删除 `GetTaskAdaptor` 函数与 `taskali`/`taskdoubao`/`taskGemini`/`hailuo`/`taskjimeng`/`kling`/`tasksora`/`suno`/`taskvertex`/`taskVidu`/`jimeng` import；删除 `GetAdaptor` 的 `APITypeJimeng` case（`&jimeng.Adaptor{}`）；`GetTaskPlatform` 也删除（先 grep 引用确认无残留）
- `relaykit/types/relay_format.go`：删除 `RelayFormatTask`
- `relay/common/relay_info.go:562`：删除 `case types.RelayFormatTask:` 分支

Run: `cd relaykit && cmd /c "set GOWORK=off&&go build ./..."` 验证 relaykit

- [ ] **Step 2: 删除 controller 层任务代码**

- 删除 `controller/task.go`（GetAllTask/GetUserTask/tasksToDto）
- `controller/relay.go`：删除 `RelayTaskFetch`、`RelayTask`、`respondTaskError`、`shouldRetryTaskRelay` 及 `taskdto` import；保留 Midjourney 分支（`RelayMidjourneyTask`/`RelayMidjourneyTaskImageSeed`）
- `router/api-router.go`：删除 taskRoute 块（`/api/task`）
- `router/relay-router.go`：删除 `relaySunoRouter` 块；保留 mjRouter 与 `registerMjRouterGroup`
- `router/video-router.go`：整文件删除；`router/main.go` 删除 `SetVideoRouter(router)` 调用
- 删除 `middleware/kling_adapter.go`、`middleware/jimeng_adapter.go`（确认无其他引用）
- 删除 `controller/video_proxy.go`、`controller/video_proxy_gemini.go`（VideoProxy 为任务内容代理）
- `controller/system_task_handlers.go`：删除 `RunTaskPollingOnce` 相关系统任务 handler（`CreateTaskPollingSystemTask` 之类，grep `TaskPolling` 定位）

- [ ] **Step 3: 删除 service 层任务代码**

- 删除 `service/task.go`、`service/task_billing.go`、`service/task_polling.go` 及测试文件
- `main.go`：删除 `GetTaskAdaptorFunc` 工厂赋值块（137-138）及 `constant` import 如无其他使用

- [ ] **Step 4: 清理 dto 与 constant**

- 删除 `dto/task.go`（TaskError/TaskData/TaskResponse/TaskDto/FetchReq，Midjourney 不使用）
- `constant/task.go`：删除 `TaskPlatformSuno`、`SunoActionMusic/Lyrics`、`TaskAction*`（检查 `TaskAction*` 是否被 Midjourney 使用——`grep TaskActionGenerate` 确认；若仅 Suno/任务用则删）、`SunoModel2Action`；保留 `TaskPlatform` 类型与 `TaskPlatformMidjourney`

- [ ] **Step 5: 编译修复（迭代）**

Run: `go build ./...` 反复直到通过
- 处理所有引用残留（`LogTaskConsumption`、`RefundTaskQuota`、`RecalculateTaskQuota`、`model.InitTask` 调用点等）
- `middleware/distributor.go`：任务分支清理（grep `task` 逐处判断；`GetByTaskId` 模型名解析保留，其调用的 `model.GetByTaskId` 在 model/task.go 保留）

- [ ] **Step 6: 验证**

Run: `go build ./... && go vet ./... && go test ./controller/... ./service/... ./relay/... ./router/... -count=1`
Expected: 成功（预存不稳定测试除外）

- [ ] **Step 7: 提交（不执行）**

---

### Task 4: 渠道特判与 billing 清理

**Files:**
- Modify: `controller/channel.go`、`controller/channel-test.go`、`controller/channel_upstream_update.go`、`controller/channel-billing.go`
- Modify: `model/pricing_default.go`
- Modify: `relay/channel/` 保留渠道内对被删渠道的引用（如 openai/adaptor.go 的 AWS 分支等，grep 定位）
- Modify: `middleware/distributor.go`、`common/`（残留引用）

**Interfaces:**
- Consumes: Task 1-3
- Produces: 无被删渠道残留分支

- [ ] **Step 1: 清理 controller 特判**

- `controller/channel.go`：删除 Ollama 特判（4 处：~2005/2068/2150/2199）、VertexAi 特判（503/519/635/660/1031，Vertex 已删）、其他被删渠道特判
- `controller/channel-test.go`：删除 Perplexity/Cohere/AWS 等被删渠道的测试分支与模型探测逻辑
- `controller/channel_upstream_update.go`：删除被删渠道的 case（Ali/Zhipu_v4/VolcEngine/Moonshot 保留；Ollama/Gemini 特判中 Gemini 保留、Ollama 删）
- `controller/channel-billing.go`：删除 `ChannelTypeAIProxy`/`ChannelTypeAPI2GPT`/`ChannelTypeAIGC2D`/`ChannelTypeOpenRouter`/`ChannelTypePerplexity`/`ChannelTypeMoonshot`（Moonshot 保留！）等 case——只删被删渠道与死常量的 case；同时删除注释死代码（`//case common.ChannelTypeOpenAISB:` 等）
- `controller/model.go`：确认 `init()` 渠道枚举无需改；删除被删渠道的模型列表来源引用（如有）

- [ ] **Step 2: 清理模型 vendor 规则**

- `model/pricing_default.go`：删除被删渠道的 vendor 规则条目（`grep -n "aws\|ollama\|cohere\|mistral\|perplexity\|grok\|replicate\|dify\|coze\|submodel\|palm\|vertex\|bedrock\|suno\|kling\|vidu\|sora\|jimeng\|veo\|hailuo\|doubao"` 逐个判断，保留 doubao LLM 相关（volcengine 保留））

- [ ] **Step 3: 清理保留渠道内引用**

- `relay/channel/openai/adaptor.go` 等保留渠道：删除对被删渠道的引用（如 AWS Bedrock 分支、`relay/channel/aws` 相关 import）
- `relay/channel/adapter.go`：`ConvertRerankRequest`/`ConvertAudioRequest` 接口方法**保留**（实现已随渠道删除）
- grep 全仓 `github.com/QuantumNous/new-api/relay/channel/aws\|...cohere\|...perplexity` 确认无残留 import

- [ ] **Step 4: 验证**

Run: `go build ./... && go vet ./... && go test ./controller/... ./model/... ./relay/... -count=1`
Expected: 成功

- [ ] **Step 5: 提交（不执行）**

---

### Task 5: 模型列表与倍率清理

**Files:**
- Modify: `setting/ratio_setting/model_ratio.go`、`setting/ratio_setting/cache_ratio.go`
- Modify: `relay/channel/` 保留渠道的模型常量（grep 确认无被删渠道模型混入）
- Modify: `model/` 模型列表相关（`controller/model.go` 的渠道模型映射）

**Interfaces:**
- Consumes: Task 1-4
- Produces: 倍率表无被删渠道模型条目

- [ ] **Step 1: 清理 model_ratio.go / cache_ratio.go**

grep 被删渠道模型前缀并删除对应倍率条目：
- 海外渠道：`palm*`、`vertex*`、`gemini-*`（仅删除 `gemini-2.5-flash-preview-tts` 等已被删的能力？Gemini 保留——只删明确属于被删能力的条目）、`mistral*`、`cohere*`、`perplexity*`、`grok-*`（xAI）、`replicate*`、`dify*`、`coze*`、`submodel*`、`ollama*`、`openrouter*`、`aws*`、`bedrock*`、`xinference*`
- 任务模型：`suno-*`、`kling-*`、`jimeng-*`、`vidu-*`、`sora-*`、`veo-*`、`hailuo-*`、`wan-*`（仅任务模型，doubao 的 `doubao-seed-*` 视频模型删、LLM 保留）、`doubao*` 视频模型
- 注意区分：`gpt-*`、`claude-*`、`gemini-*`（保留渠道模型）绝不误删

- [ ] **Step 2: 核对保留渠道模型常量**

- grep `relay/channel/openai/constant.go` 等保留渠道：确认无被删渠道专属模型（如 `kling`、`suno` 在 openai 常量中？如有删）
- `controller/model.go` 渠道模型映射（如 `GetChannelModelList` 之类）：确认被删渠道无模型来源

- [ ] **Step 3: 验证**

Run: `go build ./... && go test ./setting/... ./model/... -count=1`
Expected: 成功

- [ ] **Step 4: 提交（不执行）**

---

### Task 6: 前端清理

**Files:**
- Modify: `web/src/features/channels/constants.ts`、`web/src/features/channels/lib/channel-utils.ts`、`web/src/features/channels/lib/advanced-custom.ts`、`web/src/features/channels/components/dialogs/channel-test-dialog.tsx`
- Modify: `web/src/features/pricing/constants.ts`、`web/src/features/pricing/lib/mock-stats.ts`、`web/src/features/pricing/components/model-details-api.tsx`
- Modify: `web/src/features/models/constants.ts`、`web/src/features/models/lib/model-categories.ts`、`web/src/features/usage-logs/components/model-badge.tsx`
- Modify: `web/src/features/usage-logs/`（task 分类：`components/task-logs-filter-bar.tsx` 删、`components/columns/task-logs-columns.tsx` 删、`components/dialogs/audio-preview-dialog.tsx` 删、`components/usage-logs-mobile-card.tsx` 的 TaskLogsCard 清理、`components/usage-logs-table.tsx` 的引用清理、`types.ts` 的 TaskLog 清理、`section-registry.tsx` 的 task 分支清理、`constants.ts` 的日志分类清理）
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（经脚本）

**Interfaces:**
- Consumes: 后端常量删除结果（前端渠道类型编号对应）
- Produces: 无被删渠道 UI 入口

- [ ] **Step 1: 渠道类型映射清理**

- `web/src/features/channels/constants.ts`：删除被删渠道的类型映射（PaLM/OpenRouter/Perplexity/AWS/Cohere/SunoAPI/Dify/Cloudflare/VertexAI/Mistral/Xinference/xAI/Coze/Kling/Jimeng/Vidu/Submodel/DoubaoVideo/Sora/Replicate + Midjourney 死条目——Midjourney 前端条目若引用保留的后端逻辑则保留，先 grep 前端 `Midjourney` 引用确认）
- `channel-utils.ts`、`advanced-custom.ts`：删除对应渠道逻辑
- `channel-test-dialog.tsx`：删除被删渠道的测试选项（如 `aws`、`cohere` 端点、Suno/Kling 任务端点）

- [ ] **Step 2: 模型与定价清理**

- `pricing/mock-stats.ts`、`pricing/constants.ts`、`pricing/components/model-details-api.tsx`：删除被删渠道模型条目（grok/suno/kling/cohere/mistral/perplexity/replicate/dify/coze/vertex 等）
- `models/constants.ts`、`model-categories.ts`、`model-badge.tsx`：删除被删渠道模型分类/徽章

- [ ] **Step 3: 任务日志视图删除**

- 删除 `usage-logs/components/task-logs-filter-bar.tsx`、`components/columns/task-logs-columns.tsx`、`components/dialogs/audio-preview-dialog.tsx`
- `usage-logs-mobile-card.tsx`：删除 `TaskLogsCard` 与 task 分支
- `usage-logs-table.tsx`：删除 TaskLogsFilterBar/useTaskLogsColumns 引用
- `types.ts`：删除 TaskLog 相关类型（保留 DrawingLog 等 Midjourney 类型）
- `section-registry.tsx`/`constants.ts`：删除 task 日志分类；日志分类保留 chat/image/audio/mj（如有）
- 检查是否有 tasks 管理页面/路由引用（grep `TaskLog` 全前端）

- [ ] **Step 4: i18n 处理**

- 用一次性 Node 脚本（参考上轮删除 Rerank/Jina 键的脚本模式）：从 7 个 locale 文件删除被删渠道名键（`"PaLM"`、`"OpenRouter"`、`"Perplexity"`、`"AWS"`、`"Cohere"`、`"Suno"`、`"Dify"`、`"Cloudflare"`、`"VertexAI"`、`"Mistral"`、`"xAI"`、`"Coze"`、`"Kling"`、`"Jimeng"`、`"Vidu"`、`"Submodel"`、`"DoubaoVideo"`、`"Sora"`、`"Replicate"`、`"Ollama"`、`"Xinference"`、任务相关键）与 `static-keys.ts` 同步
- 脚本执行后删除脚本文件
- 注意：删除键后运行 `bun run i18n:sync` 确认无新缺失

- [ ] **Step 5: 前端验证**

Run: `bun run typecheck && bun run test && bun run build`
Expected: 成功（lint 仅确认改动文件无新增错误）

- [ ] **Step 6: 提交（不执行）**

---

### Task 7: 全量验证与单次提交

**Files:**
- 全仓库改动

**Interfaces:**
- Consumes: Task 1-6 全部完成

- [ ] **Step 1: 后端全量验证**

Run:
```
go build ./...
go vet ./...
go test ./... -count=1
```
Expected: 仅 2 个预存不稳定测试失败（`service` 的 channel_affinity 2 个）；若 relay/channel 的 TestUpstreamGetBody 失败属预存抖动，重跑确认

- [ ] **Step 2: relaykit 独立构建**

Run: `cd relaykit && cmd /c "set GOWORK=off&&go build ./..."`
Expected: 成功

- [ ] **Step 3: gofmt 与 go mod tidy**

Run: `gofmt -l .`（排除 web/、.codegraph/；预存不洁 `controller/misc.go` 除外）与 `go mod tidy`
Expected: 无新增不洁文件；go.mod 无被删渠道专属依赖需清理（如有则清理）

- [ ] **Step 4: codegraph 复查**

Run: `codegraph sync && codegraph query "ChannelTypeCohere" && codegraph query "GetTaskAdaptor" && codegraph query "RelayTaskSubmit" && codegraph query "TaskPlatformSuno" && codegraph query "VideoProxy" && codegraph query "RelayFormatTask"`
Expected: 全零残留（子串模糊匹配命中的无关符号除外）

- [ ] **Step 5: findstr 最终扫描**

Run（排除 web/dist、node_modules、docs/、.codegraph/）：
```
findstr /s /m /c:"ChannelTypeCohere" /c:"ChannelTypeMistral" /c:"ChannelTypeOllama" /c:"ChannelTypeSunoAPI" /c:"ChannelTypeKling" /c:"ChannelTypeSora" /c:"ChannelTypeAws" /c:"ChannelTypeXai" /c:"GetTaskAdaptor" /c:"RelayTaskSubmit" /c:"TaskPlatformSuno" /c:"RelayFormatTask" /c:"ConvertRerankRequest" /c:"ConvertAudioRequest" /c:"cfSTTHandler" *.go
```
Expected: `ConvertRerankRequest`/`ConvertAudioRequest` 有命中（接口方法保留，属预期），其余零命中

- [ ] **Step 6: 单次提交**

Run:
```
git add -u
git status --short   // 确认无意外文件（.codegraph/、docs/learn/、opencode.json 保持 untracked）
git add -f docs/superpowers/plans/2026-08-16-remove-channel-providers.md
git commit --no-gpg-sign -m "refactor(channel): remove unused channel providers and task platforms

- Remove 15 channel adapters (PaLM, VertexAI, AWS Bedrock, Cohere, Mistral,
  Perplexity, xAI, Replicate, Ollama, OpenRouter, Cloudflare, Coze, Dify,
  Submodel, Xinference) and 10 task platforms (Suno, Kling, Jimeng, Vidu,
  Sora, DoubaoVideo, hailuo, Ali/Gemini/Vertex tasks)
- Remove task pipeline (relay_task, task polling/billing, video/task routes,
  task log UI); keep tasks table and Task model for historical data
- Remove legacy dead ChannelType constants (Midjourney, AIProxy, FastGPT,
  etc.) and stale billing cases; iota numbering shifts
- Keep chat/image/embedding pipelines, Midjourney proxy, and audio-token
  billing shared infrastructure"
```

Commit message 参照上轮协议删除提交风格（如上），可微调措辞

- [ ] **Step 7: 收尾报告**

汇报用户：改动统计、验证结果、预存不稳定测试说明、保留项确认（任务表/历史数据/Midjourney）
