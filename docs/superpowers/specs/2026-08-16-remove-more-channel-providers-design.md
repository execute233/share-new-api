# Design Spec: 删除第三批渠道供应商（15 个国产/海外渠道）

日期：2026-08-16
状态：已批准（方案 A）

## 背景与目标

new-api 已完成两轮渠道删除（协议/任务链路清理、15 个海外渠道 + 10 个任务平台删除）。本项目延续相同模式，删除剩余的 15 个国产/海外渠道，将渠道供应商收敛到以 OpenAI/Anthropic/Gemini 为核心的 9 个类型。

删除后中转站仅保留：OpenAI(1)、Anthropic(14)、Gemini(24)、Custom(8)、Codex(57)、AdvancedCustom(58)、Sub2API(59)、NewAPI(60)、Dummy(61)。

## 删除范围

以下 15 个渠道名（共 16 个渠道类型值，Zhipu 含 16/26 两个常量）及其适配器、特判、前端选项、i18n、倍率与计费条目全部删除：

| 渠道 | ChannelType 值 | APIType | 适配器目录 |
|---|---|---|---|
| Azure | 3 | (OpenAI 内嵌) | 无（嵌于 openai/） |
| Baidu | 15 | APITypeBaidu | baidu/ |
| Zhipu | 16 | APITypeZhipu | zhipu/ |
| Ali | 17 | APITypeAli | ali/ |
| Xunfei | 18 | APITypeXunfei | xunfei/ |
| 360 | 19 | (OpenAI 内嵌) | ai360/（仅死常量） |
| Tencent | 23 | APITypeTencent | tencent/ |
| Moonshot | 25 | APITypeMoonshot | moonshot/ |
| ZhipuV4 | 26 | APITypeZhipuV4 | zhipu_4v/ |
| LingYiWanWu | 31 | (OpenAI 内嵌) | lingyiwanwu/（仅死常量） |
| MiniMax | 35 | APITypeMiniMax | minimax/ |
| SiliconFlow | 40 | APITypeSiliconFlow | siliconflow/ |
| DeepSeek | 43 | APITypeDeepSeek | deepseek/ |
| MokaAI | 44 | APITypeMokaAI | mokaai/ |
| VolcEngine | 45 | APITypeVolcEngine | volcengine/ |
| BaiduV2 | 46 | APITypeBaiduV2 | baidu_v2/ |

用户已确认：Zhipu(16) 与 ZhipuV4(26) **两个都删**；保留清单（OpenAI/Anthropic/Gemini/Custom/Codex/AdvancedCustom/Sub2API/NewAPI）全部确认保留。

## 关键约束

1. **ChannelType 数值断号不动**：`channels.type` 列在 DB 中持久化，删除常量后不得重排其他常量的值（沿用前两轮规则）。`ChannelBaseURLs` 保持 61 项占位数组，被删位置置 `""`。`ChannelTypeDummy = 61` 保持最后。
2. **APIType 删除规则**：APIType 常量使用 iota 压缩（非 DB 持久化，前轮已压缩）；被删条目直接移除，保留 `_ // APITypeJimeng` 注释断号模式。`common.ChannelType2APIType` 映射同步清理（被删 ChannelType 不再映射到 APIType）。
3. **relaykit 独立性**：relaykit/ 若受影响必须 `cd relaykit && GOWORK=off go build ./...` 验证；倾向不触碰 relaykit 导出 API。
4. **JSON/DB 兼容**：本计划无 DB 迁移；删除的是代码常量与适配器，存量渠道数据保留在库中（列表页按 Unknown 显示）。

## 分层改动

### 1. 常量层（constant/channel.go、constant/api_type.go、common/api_type.go）

- `constant/channel.go`：删除上述 16 个 ChannelType 常量；`ChannelBaseURLs` 被删位置置 `""`；`ChannelTypeNames` 删除对应 16 个条目；**删除 `ChannelSpecialBases` 全表**（glm-coding-plan/glm-coding-plan-international/kimi-coding-plan/doubao-coding-plan——其域名分别属于 Zhipu/Moonshot/VolcEngine，渠道删除后无使用者）。
- `constant/api_type.go`：删除 13 个 APIType 常量（Baidu/Zhipu/Ali/Xunfei/Tencent/ZhipuV4/SiliconFlow/DeepSeek/MokaAI/VolcEngine/BaiduV2/Moonshot/MiniMax），保留 `_` 断号模式。
- `common/api_type.go`：`ChannelType2APIType`/`APIType2ChannelType`（如存在）删除被删映射；同时确认 `IsSupportedChannelType` 类校验函数更新。

### 2. 适配器层

- 删除 15 个目录：`relay/channel/ali/`、`baidu/`、`baidu_v2/`、`deepseek/`、`minimax/`、`mokaai/`、`moonshot/`、`siliconflow/`、`tencent/`、`volcengine/`、`xunfei/`、`zhipu/`、`zhipu_4v/`、`ai360/`、`lingyiwanwu/`（后两个仅含死常量文件，一并删除）。
- `relay/relay_adaptor.go`：删除 13 个 APIType case 与对应 import。
- **`relay/channel/openai/adaptor.go` 保留但清理内嵌分支**（Azure/360/LingYiWanWu 走 APITypeOpenAI）：
  - `:101` 的 `case constant.ChannelTypeAzure`（ChannelType→APIType 映射）
  - `:165`、`:193` 的 `info.ChannelType == constant.ChannelTypeAzure` 特判
  - `:457-470` 的 `case constant.ChannelType360` / `case constant.ChannelTypeLingYiWanWu` 特判
  - `:147` 注释掉的 MiniMax case 一并清理
- `relay/common/relay_utils.go:26-28`：删除 `case constant.ChannelTypeAzure` 分支。
- `relay/common/relay_info.go`：`:195` Azure 特判、`streamSupportedChannels`（:305-318）删除 Azure/VolcEngine/DeepSeek/BaiduV2/Zhipu_v4/Ali/Moonshot/MiniMax/SiliconFlow/Tencent 条目，保留 OpenAI/Anthropic/Gemini/Codex/AdvancedCustom/Sub2API/NewAPI。
- `relay/channel/openai/usage.go`：删除 DeepSeek/Zhipu_v4/Moonshot usage 特判分支（保留 OpenAI case 与默认路径）。

### 3. 控制器层

- `controller/channel-billing.go`：`updateChannelBalance` 删除 Azure（未实现死分支）、SiliconFlow、DeepSeek、Moonshot case 及对应 helper（`updateChannelSiliconFlowBalance`/`updateChannelDeepSeekBalance`/`updateChannelMoonshotBalance`）；检查文件中其他被删渠道引用。
- `controller/channel-test.go`：`:112` MokaAI embedding 测试特判、`:117` VolcEngine seedream 特判删除（Codex 特判保留）。
- `controller/channel_upstream_update.go`：删除 `ChannelSpecialBases` 相关分支（:366-378）。
- `middleware/distributor.go`：`:380` Azure、`:386` Ali 特判删除。
- `controller/channel_test_internal_test.go`、`controller/model_owned_by_test.go`：更新被删渠道相关断言。

### 4. 计费与倍率

- `setting/ratio_setting/model_ratio.go`：删除被删渠道模型倍率条目（glm/deepseek/moonshot/kimi/ark/doubao/qwen/ernie/seedream 等被删渠道专属模型）；保留 OpenAI/Claude/Gemini 模型。
- `model/pricing_default.go`：清理被删渠道 vendor 规则与图标。
- `setting/ratio_setting/ratio_sync.go` 等：全仓扫描被删渠道专属模型/域名引用并清理（实现时以 findstr 扫描为准，逐条判断）。

### 5. 前端

- `web/src/features/channels/constants.ts`：`CHANNEL_TYPES` 删除 15 个条目（3/15/16/17/18/19/23/25/26/31/35/40/43/44/45/46）；`CHANNEL_TYPE_DISPLAY_ORDER` 移除被删 id；`MODEL_FETCHABLE_TYPES` 移除 17/23/25/26/31/35/40/43。
- `web/src/features/channels/lib/channel-utils.ts`：`TYPE_TO_ICON` 删除被删渠道映射。
- `web/src/features/channels/lib/channel-type-config.ts`：删除 3（Azure）、43（DeepSeek）配置块。
- `web/src/features/channels/lib/channel-form.ts`：删除 Tencent key format hint（:390 附近）及其他被删渠道表单特判（实现时 findstr 扫描）。
- i18n：删除 7 个 locale 中 15 个渠道名键及关联键（参照前轮一次性脚本模式）；`static-keys.ts` 同步。
- 其他：全仓 findstr 扫描被删渠道引用（模型下拉、mock 数据、测试夹具）。

### 6. 文档与杂项

- `docs/`、OpenAPI 示例等含被删渠道的文档条目清理（可延迟项：纯文档残留不阻塞合并，记录 ledger）。
- go.mod 依赖清理：`go mod tidy` 移除被删渠道专属依赖（若有，如 tencent/baidu SDK 类）。

## 边界与例外

- **保留**：OpenAI(1)/Anthropic(14)/Gemini(24)/Custom(8)/Codex(57)/AdvancedCustom(58)/Sub2API(59)/NewAPI(60)/Dummy(61)；openai/claude/gemini/codex/newapi/sub2api/advancedcustom 适配器；tasks 表与 Task 模型；Midjourney 链路（2/5 常量、mjproxy、drawing 日志）不动。
- **Custom(8) 保留**：通用自定义渠道不在删除清单。
- **存量数据**：DB 中 type=3/15/16/17/18/19/23/25/26/31/35/40/43/44/45/46 的存量渠道保留，列表显示 Unknown（`GetChannelTypeName` 回退）；不提供迁移。
- **编码套餐映射**：`ChannelSpecialBases`（glm/kimi/doubao coding-plan）全部删除——其唯一使用者是 ZhipuV4/Moonshot/VolcEngine 适配器与上游模型发现路径，渠道删除后为死代码。
- **部署配置**：被删渠道无专属 env（前轮已确认任务 env 删除模式）；如有发现记录 ledger。

## 验证方案

1. `go build ./... && go vet ./... && go test ./... -count=1`（预期仅 2 个预存不稳定测试：service channel_affinity）
2. `cd relaykit && GOWORK=off go build ./...`
3. 前端：`bun run typecheck && bun run test && bun run build`
4. codegraph sync + 被删符号零残留查询（ChannelTypeAli/ChannelTypeAzure/ChannelTypeDeepSeek/APITypeMoonshot/ChannelSpecialBases 等）
5. findstr 最终扫描（被删常量/目录名/域名零命中；`ConvertRerankRequest` 类接口方法名不适用本批）
6. 全量 gofmt 检查（无新增不洁文件）

## 交付

- 沿用 SDD 流程：spec → plan → subagent 分任务执行 → 逐任务审查 → 全量验证 → **单次提交**（squash 所有 task 提交）。
- 提交信息主题：`refactor(channel): remove 15 more channel providers (Azure, Chinese providers...)`。
- spec 与 plan 文档提交在 `docs/superpowers/specs/` 与 `docs/superpowers/plans/`。
