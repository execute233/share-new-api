# 删除第三批渠道供应商（16 个渠道类型值）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除 15 个渠道名（16 个渠道类型值）的适配器、特判、前端选项、i18n、倍率与计费条目，仅保留 9 个渠道类型。

**Architecture:** 分层删除：常量层（ChannelType/APIType，值断号不动）→ 适配器层（15 个目录 + openai 内嵌的 Azure/360/LingYiWanWu 分支）→ 控制器特判 → 计费/倍率 → 前端 → i18n。延续前两轮已确立的删除模式与验证流程。

**Tech Stack:** Go 1.22+（Gin/GORM）、React 19（Bun/Rsbuild/i18next）。

## Global Constraints

- **ChannelType 数值断号不动**：删除常量后 OpenAI=1/Custom=8/Anthropic=14/Gemini=24/Codex=57/AdvancedCustom=58/Sub2API=59/NewAPI=60/Dummy=61 保持不变；`ChannelBaseURLs` 保持 61 项占位数组，被删位置置 `""`；`ChannelTypeDummy=61` 保持最后。
- **被删清单**：Azure(3)、Baidu(15)、Zhipu(16)、Ali(17)、Xunfei(18)、360(19)、Tencent(23)、Moonshot(25)、Zhipu_v4(26)、LingYiWanWu(31)、MiniMax(35)、SiliconFlow(40)、DeepSeek(43)、MokaAI(44)、VolcEngine(45)、BaiduV2(46)。
- **保留清单**：OpenAI(1)、Anthropic(14)、Gemini(24)、Custom(8)、Codex(57)、AdvancedCustom(58)、Sub2API(59)、NewAPI(60)、Dummy(61)。
- **`ChannelSpecialBases` 全表删除**（glm/kimi/doubao coding-plan——Zhipu/Moonshot/VolcEngine 域名）。
- **Azure 无独立 APIType**：Azure 走 APITypeOpenAI，其特判嵌在 openai 适配器与 relay/common 中，必须逐处清理。
- **relaykit 独立性**：relaykit/ 改动必须 `cd relaykit && cmd /c "set GOWORK=off&&go build ./..."` 验证；本计划不触碰 relaykit 导出 API。
- **环境**（Windows/cmd）：`grep` 不可用用 `findstr`；gpg 签名超时 → 一律 `git commit --no-gpg-sign`；PowerShell 改 Go 文件后必须 `gofmt -w` + 转 LF；前端命令在 `web/` 目录用 `bun`。
- **预存不稳定测试**（非本计划引入，最终验证需知晓）：`service` 的 channel_affinity 2 个（时间戳碰撞）；relay/channel `TestUpstreamGetBody` HTTP2 系列（抖动，重跑确认）。
- **预存 gofmt 不洁**：仅 `controller/misc.go`（不动）。
- **存量数据**：DB 中 type=3/15/16/17/18/19/23/25/26/31/35/40/43/44/45/46 的存量渠道保留在库，列表显示 Unknown；无迁移。
- **提交**：每个任务完成后提交（`git commit --no-gpg-sign`）；最终控制器 squash 为单次提交。
- **测试纪律**：删除渠道必然删除其测试文件；保留文件中被删渠道的断言须同步更新，`go test` 不得因本计划出现新失败。

---

### Task 1: 常量层删除（ChannelType/APIType/映射/ChannelSpecialBases）

**Files:**
- Modify: `constant/channel.go`
- Modify: `constant/api_type.go`
- Modify: `common/api_type.go`

**Interfaces:**
- Consumes: 无
- Produces: 删除后的 `constant.ChannelTypeOpenAI=1 / ChannelTypeCustom=8 / ChannelTypeAnthropic=14 / ChannelTypeGemini=24 / ChannelTypeCodex=57 / ChannelTypeAdvancedCustom=58 / ChannelTypeSub2API=59 / ChannelTypeNewAPI=60 / ChannelTypeDummy=61`（值不变）；`common.ChannelType2APIType` 只含保留渠道映射。

- [ ] **Step 1: 删除 ChannelType 常量**

`constant/channel.go`（当前行号）删除以下常量行：
- :6 `ChannelTypeAzure = 3`
- :9 `ChannelTypeBaidu = 15`
- :10 `ChannelTypeZhipu = 16`
- :11 `ChannelTypeAli = 17`
- :12 `ChannelTypeXunfei = 18`
- :13 `ChannelType360 = 19`
- :14 `ChannelTypeTencent = 23`
- :16 `ChannelTypeMoonshot = 25`
- :17 `ChannelTypeZhipu_v4 = 26`
- :18 `ChannelTypeLingYiWanWu = 31`
- :19 `ChannelTypeMiniMax = 35`
- :20 `ChannelTypeSiliconFlow = 40`
- :21 `ChannelTypeDeepSeek = 43`
- :22 `ChannelTypeMokaAI = 44`
- :23 `ChannelTypeVolcEngine = 45`
- :24 `ChannelTypeBaiduV2 = 46`

保留：Unknown(0)/OpenAI(1)/Custom(8)/Anthropic(14)/Gemini(24)/Codex(57)/AdvancedCustom(58)/Sub2API(59)/NewAPI(60)/Dummy(61)。**不得改任何保留常量的值。**

- [ ] **Step 2: ChannelBaseURLs 被删位置置空**

`ChannelBaseURLs` 数组（61 项）中被删索引置 `""`：索引 3、15、16、17、18、19、23、25、26、31、35、40、43、44、45、46。保留 1=OpenAI、14=Anthropic、24=Gemini、57=Codex 的 URL；索引 0/2/5-13 等已有 `""` 的保持不动。数组长度保持 61。

- [ ] **Step 3: 删除 ChannelTypeNames 条目**

`ChannelTypeNames` map 删除 16 个条目：Azure/Baidu/Zhipu/Ali/Xunfei/360/Tencent/Moonshot/ZhipuV4/LingYiWanWu/MiniMax/SiliconFlow/DeepSeek/MokaAI/VolcEngine/BaiduV2。

- [ ] **Step 4: 删除 ChannelSpecialBases 全表**

删除 `ChannelSpecialBases` 类型定义（`ChannelSpecialBase` struct + 4 个 map 条目，:132-153）。

- [ ] **Step 5: 删除 APIType 常量**

`constant/api_type.go` 删除：`APITypeBaidu`、`APITypeZhipu`、`APITypeAli`、`APITypeXunfei`、`APITypeTencent`、`APITypeZhipuV4`、`APITypeSiliconFlow`、`APITypeDeepSeek`、`APITypeMokaAI`、`APITypeVolcEngine`、`APITypeBaiduV2`、`APITypeMoonshot`、`APITypeMiniMax`（iota 自动重排后续值，允许；保留 `_ // APITypeJimeng (removed, numbering preserved)` 注释模式）。保留：APITypeOpenAI/Anthropic/Gemini/Codex/AdvancedCustom/Sub2API/NewAPI/Dummy。

- [ ] **Step 6: 清理 ChannelType2APIType**

`common/api_type.go` `ChannelType2APIType` 删除 13 个 case（Baidu/Zhipu/Ali/Xunfei/Tencent/Zhipu_v4/SiliconFlow/DeepSeek/MokaAI/VolcEngine/BaiduV2/Moonshot/MiniMax）。`SupportsResponsesCompact` 不动（只引用保留 APIType）。删除后 `go build ./...` 必须通过——若其他文件引用被删常量，记录到 Task 2 清理清单（不得在本任务修复，避免越界；但 `channel_test_internal_test.go` 等仅测试文件可在 Task 2）。

- [ ] **Step 7: 验证与提交**

```bash
go build ./... 2>&1 | Select-Object -First 30
```
记录所有引用被删常量的文件清单（供 Task 2 使用，必须零遗漏）。
```bash
git add constant/channel.go constant/api_type.go common/api_type.go && git commit --no-gpg-sign -m "refactor(constant): remove 16 channel types and ChannelSpecialBases (task 1)"
```

---

### Task 2: 适配器层与后端特判清理

**Files:**
- Delete: `relay/channel/ali/`、`baidu/`、`baidu_v2/`、`deepseek/`、`minimax/`、`mokaai/`、`moonshot/`、`siliconflow/`、`tencent/`、`volcengine/`、`xunfei/`、`zhipu/`、`zhipu_4v/`、`ai360/`、`lingyiwanwu/`
- Modify: `relay/relay_adaptor.go`
- Modify: `relay/channel/openai/adaptor.go`
- Modify: `relay/channel/openai/usage.go`
- Modify: `relay/common/relay_utils.go`
- Modify: `relay/common/relay_info.go`
- Modify: `middleware/distributor.go`
- Modify: `controller/channel-billing.go`
- Modify: `controller/channel-test.go`
- Modify: `controller/channel_upstream_update.go`
- Modify: `controller/channel_test_internal_test.go`
- Modify: `controller/model_owned_by_test.go`

**Interfaces:**
- Consumes: Task 1 删除后的常量
- Produces: 全仓库对 16 个被删 ChannelType/13 个被删 APIType 的引用为零（`findstr` 验证）；openai 适配器仅含保留渠道分支

- [ ] **Step 1: 删除 15 个适配器目录**

```powershell
Remove-Item -Recurse -Force relay\channel\ali, relay\channel\baidu, relay\channel\baidu_v2, relay\channel\deepseek, relay\channel\minimax, relay\channel\mokaai, relay\channel\moonshot, relay\channel\siliconflow, relay\channel\tencent, relay\channel\volcengine, relay\channel\xunfei, relay\channel\zhipu, relay\channel\zhipu_4v, relay\channel\ai360, relay\channel\lingyiwanwu
```
（含 `tencent/dispatch_test.go`、`ali/adaptor_test.go` 等随目录删除。）

- [ ] **Step 2: 清理 GetAdaptor**

`relay/relay_adaptor.go` 删除 13 个 case（APITypeAli/Baidu/Tencent/Xunfei/Zhipu/ZhipuV4/SiliconFlow/DeepSeek/MokaAI/VolcEngine/BaiduV2/Moonshot/MiniMax）与 13 个对应 import（ali/baidu/baidu_v2/deepseek/minimax/mokaai/moonshot/siliconflow/tencent/volcengine/xunfei/zhipu/zhipu_4v）。

- [ ] **Step 3: 清理 openai 适配器内嵌分支**

`relay/channel/openai/adaptor.go`：
- 删除 `case constant.ChannelTypeAzure:`（ChannelType→APIType 映射 switch 中，当前 :101）
- 删除 `//case constant.ChannelTypeMiniMax:` 注释行（:147）
- 删除 `info.ChannelType == constant.ChannelTypeAzure` 的两个特判分支（:165、:193 附近，连同其独立函数/分支体）
- 删除 `case constant.ChannelType360:` 与 `case constant.ChannelTypeLingYiWanWu:` 的分支（:457-470 附近，含 360/LingYiWanWu 专属 base URL 特判）

- [ ] **Step 4: 清理 usage 特判**

`relay/channel/openai/usage.go` 删除 `case constant.ChannelTypeDeepSeek:` / `case constant.ChannelTypeZhipu_v4:` / `case constant.ChannelTypeMoonshot:` 分支（:16-44 附近），保留 OpenAI case 与默认路径。

- [ ] **Step 5: 清理 relay/common 特判**

- `relay/common/relay_utils.go`：删除 `case constant.ChannelTypeAzure:` 分支（:26-28 附近，保留 ChannelTypeOpenAI case）
- `relay/common/relay_info.go`：删除 :195 附近 `channelType == constant.ChannelTypeAzure` 特判；`streamSupportedChannels` 删除 Azure/VolcEngine/DeepSeek/BaiduV2/Zhipu_v4/Ali/Moonshot/MiniMax/SiliconFlow/Tencent 条目（保留 OpenAI/Anthropic/Gemini/Codex/AdvancedCustom/Sub2API/NewAPI）

- [ ] **Step 6: 清理控制器特判**

- `middleware/distributor.go`：删除 :380 附近 `case constant.ChannelTypeAzure:` 与 :386 附近 `case constant.ChannelTypeAli:` 分支
- `controller/channel-billing.go`：`updateChannelBalance` 删除 Azure（未实现）/SiliconFlow/DeepSeek/Moonshot case（:242-251），删除 `updateChannelSiliconFlowBalance`/`updateChannelDeepSeekBalance`/`updateChannelMoonshotBalance` helper 与相关 import
- `controller/channel-test.go`：删除 :112 附近 `channel.Type == constant.ChannelTypeMokaAI` embedding 特判、:117 附近 `channel.Type == constant.ChannelTypeVolcEngine && strings.Contains(testModel, "seedream")` 特判（Codex 特判 :49/:625 保留）
- `controller/channel_upstream_update.go`：删除 :366-378 附近 `ChannelSpecialBases` 相关分支（3 处 `if plan, ok := constant.ChannelSpecialBases[baseURL]...`）

- [ ] **Step 7: 清理测试断言**

- `controller/channel_test_internal_test.go`、`controller/model_owned_by_test.go`：删除/更新被删渠道相关测试数据与断言（如 ownedBy 期望值、渠道类型用例）
- 全仓扫描：`findstr /s /m /c:"ChannelTypeAzure" /c:"ChannelTypeBaidu" /c:"ChannelTypeZhipu" /c:"ChannelTypeAli" /c:"ChannelTypeXunfei" /c:"ChannelType360" /c:"ChannelTypeTencent" /c:"ChannelTypeMoonshot" /c:"ChannelTypeLingYiWanWu" /c:"ChannelTypeMiniMax" /c:"ChannelTypeSiliconFlow" /c:"ChannelTypeDeepSeek" /c:"ChannelTypeMokaAI" /c:"ChannelTypeVolcEngine" /c:"ChannelTypeBaiduV2" /c:"ChannelSpecialBases" /c:"APITypeMoonshot" /c:"APITypeMiniMax" /c:"APITypeAli" /c:"APITypeZhipu" /c:"APITypeTencent" /c:"APITypeVolcEngine" /c:"APITypeDeepSeek" /c:"APITypeBaidu" /c:"APITypeSiliconFlow" /c:"APITypeMokaAI" /c:"APITypeXunfei" /c:"APITypeBaiduV2" /c:"APITypeZhipuV4" *.go`
  预期：零命中（排除 `docs/`、`web/`、`.codegraph/`）。如有残留，判断归属后清理（属于本批渠道的引用必须删；属于保留渠道的误伤不得动）。

- [ ] **Step 8: 验证与提交**

```bash
gofmt -w <本次改动的所有 .go 文件>   # 然后转 LF（无 CRLF）
go build ./...
go vet ./...
go test ./controller/... ./relay/... ./middleware/... -count=1
```
预期：无新失败（预存 channel_affinity/HTTP2 抖动除外）。
```bash
git add -A && git commit --no-gpg-sign -m "refactor(relay): remove 16 channel adapters and backend special cases (task 2)"
```

---

### Task 3: 计费与倍率清理

**Files:**
- Modify: `setting/ratio_setting/model_ratio.go`
- Modify: `model/pricing_default.go`
- Modify: `go.mod` / `go.sum`（若有被删渠道专属依赖）

**Interfaces:**
- Consumes: Task 1-2 的常量与目录删除
- Produces: 倍率表与 vendor 映射中零被删渠道条目；`go mod tidy` 后无被删渠道专属依赖

- [ ] **Step 1: 删除被删渠道模型倍率**

`setting/ratio_setting/model_ratio.go`（`defaultModelPrice` map，约 :180-260）删除以下被删渠道专属模型段（逐段删除，保留 OpenAI/Claude/Gemini 条目与结构）：
- `glm-*`（:189-199 全部 11 条：glm-4/glm-4v/glm-4-alltools/glm-3-turbo/glm-4-plus/glm-4-0520/glm-4-air/glm-4-airx/glm-4-long/glm-4-flash/glm-4v-plus）
- `qwen-*`（:200 起，qwen-turbo/qwen-plus 等全部阿里条目）
- `hunyuan`（:208）
- `yi-*`（:211-222 全部 12 条）
- `deepseek-*`（:223-225 起，deepseek-chat/deepseek-coder/deepseek-reasoner）
- `moonshot`/`kimi-*`/`kimmoonshot` 等 Moonshot 条目（若有）
- `minimax`/`abab*` 条目（若有）
- `doubao-*`、`seedream-*`、`ernie-*`、`spark-*`、`wenxin-*`（若有）

判断标准：模型名属于被删 15 个渠道的（智谱 GLM/阿里 Qwen/腾讯混元/零一 Yi/DeepSeek/Moonshot Kimi/MiniMax/字节豆包+Seedream/百度文心/讯飞 Spark/360/硅基流动/Moka/火山方舟/Azure），全部删除；gpt/dall-e/o1/o3/claude/gemini 保留。**范围不确定的条目在报告中列出，不擅自保留。**

- [ ] **Step 2: 清理 vendor 规则与图标**

`model/pricing_default.go`：
- `defaultVendorRules` 删除：`moonshot`/`kimi`、`chatglm`/`glm-`、`qwen`、`deepseek`、`abab`/`minimax`、`ernie`、`spark`、`hunyuan`、`360`、`yi`、`doubao`（保留 gpt/dall-e/o1/o3/claude/gemini/llama；`jina` 属上轮已删渠道的残留，一并删除）
- `defaultVendorIcons` 删除：Moonshot/智谱/阿里巴巴/DeepSeek/MiniMax/百度/讯飞/腾讯/360/零一万物/字节跳动/Jina/微软/Microsoft/Azure 条目（保留 OpenAI/Anthropic/Google/Meta）

- [ ] **Step 3: go mod tidy 与残留扫描**

```bash
go mod tidy
go build ./...
```
若 tidy 移除被删渠道专属依赖（如 tencent/baidu SDK），记录清单；检查 `go.sum` 无异常新增。
扫描残留：`findstr /s /m /c:"moonshot" /c:"kimi" /c:"glm-" /c:"qwen" /c:"deepseek" /c:"minimax" /c:"ernie" /c:"hunyuan" /c:"doubao" /c:"spark" /c:"volcengine" /c:"lingyiwanwu" /c:"moka" /c:"siliconflow" /c:"xunfei" /c:"baidu" /c:"azure" *.go`——逐条判断：属于被删渠道代码引用的必须清理；属于注释/文档/保留渠道（如 azure 出现在旧测试名）的列出判断。

- [ ] **Step 4: 验证与提交**

```bash
go build ./... && go vet ./... && go test ./setting/... ./model/... -count=1
```
```bash
git add -A && git commit --no-gpg-sign -m "chore(ratio): remove ratios and vendor rules for deleted channels (task 3)"
```

---

### Task 4: 前端渠道清理

**Files:**
- Modify: `web/src/features/channels/constants.ts`
- Modify: `web/src/features/channels/lib/channel-utils.ts`
- Modify: `web/src/features/channels/lib/channel-type-config.ts`
- Modify: `web/src/features/channels/lib/channel-form.ts`
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Modify: `web/src/features/channels/types.ts`
- Test: `web/src/features/channels/lib/__tests__/new-api-channel.test.ts`（如有被删渠道断言）

**Interfaces:**
- Consumes: Task 1 的删除范围（16 个类型值）
- Produces: 前端无被删渠道的选项/配置/表单字段引用

- [ ] **Step 1: 删除 CHANNEL_TYPES 与 DISPLAY_ORDER**

`web/src/features/channels/constants.ts`：
- `CHANNEL_TYPES`（:28-54）删除 16 个键：3/15/16/17/18/19/23/25/26/31/35/40/43/44/45/46（保留 1/2/5/8/14/24/57/58/59/60）
- `CHANNEL_TYPE_DISPLAY_ORDER`（:56-59）移除被删 id
- `MODEL_FETCHABLE_TYPES`（:359-361）移除 17/23/25/26/31/35/40/43，保留 1/14/57/58/59/60

- [ ] **Step 2: 删除 TYPE_TO_ICON 映射**

`web/src/features/channels/lib/channel-utils.ts`（:48-85）删除 16 条：3(Azure)/15(Baidu)/46(Baidu)/16(Zhipu)/26(Zhipu)/17(Qwen)/18(Spark)/23(Hunyuan)/19(Ai360)/25(Moonshot)/31(Yi)/35(Minimax)/45(Volcengine)/43(DeepSeek)/40(SiliconCloud)/44(OpenAI-MokaAI)；保留 1/2/5/8/14/24/58/59/60。

- [ ] **Step 3: 删除 channel-type-config 配置块**

`web/src/features/channels/lib/channel-type-config.ts` 删除 3（Azure）与 43（DeepSeek）两个配置块（:65-75、:95-104），保留 1/14/24/58/59/60。

- [ ] **Step 4: 删除 Azure 表单字段全链**

`web/src/features/channels/lib/channel-form.ts`：删除 `azure_responses_version`（schema :255、defaults :397、transform :474/:529、buildSettingsJSON :589-593）与 Tencent key format hint（:390 附近 `23: 'Format: TokenHub API Key...'` 条目）
`web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`：删除 `azure_responses_version`（敏感字段清单 :283、formErrors :953、渲染块 :2179 附近）
`web/src/features/channels/types.ts`：删除 `azure_responses_version?: string`（:94）

- [ ] **Step 5: 测试与扫描**

`web/src/features/channels/lib/__tests__/new-api-channel.test.ts`：检查被删渠道断言（如 MODEL_FETCHABLE_TYPES 用例），更新为保留渠道。
全仓前端扫描（`web/src` 排除 `node_modules`/`dist`）：
```powershell
findstr /s /m /c:"azure_responses_version" /c:"CHANNEL_TYPE_AZURE" /c:"CHANNEL_TYPE_BAIDU" /c:"CHANNEL_TYPE_ALI" /c:"CHANNEL_TYPE_TENCENT" /c:"CHANNEL_TYPE_MOONSHOT" /c:"CHANNEL_TYPE_ZHIPU" /c:"CHANNEL_TYPE_DEEPSEEK" web\src\*.ts web\src\*.tsx
```
如有残留（如 model-categories、mock 数据中的被删渠道模型），逐条判断清理（只清被删渠道的）。

- [ ] **Step 6: 验证与提交**

```bash
bun run typecheck && bun run test
```
（`bun run build` 由 Task 6 全量跑）
```bash
git add -A && git commit --no-gpg-sign -m "refactor(web): remove deleted channel types and Azure form fields (task 4)"
```

---

### Task 5: i18n 清理

**Files:**
- Modify: `web/src/i18n/locales/en.json`、`zh.json`、`zh-TW.json`、`fr.json`、`ja.json`、`ru.json`、`vi.json`
- Modify: `web/src/i18n/static-keys.ts`（若有相关键）

**Interfaces:**
- Consumes: Task 4 删除的前端引用
- Produces: 7 个 locale 与 static-keys 零被删渠道键

- [ ] **Step 1: 找出被删渠道 i18n 键**

先列出候选键清单（在前端已删引用后仍是孤儿/或引用与被删渠道绑定）：
- 渠道名键：`Azure`、`Baidu`、`Zhipu`、`Zhipu V4`、`Ali`、`Xunfei`、`360`、`Tencent`、`Moonshot`、`LingYiWanWu`、`MiniMax`、`SiliconFlow`、`DeepSeek`、`MokaAI`、`VolcEngine`、`Baidu V2`
- 提示键：`Your Azure OpenAI endpoint URL` 等（扫描 `web/src/features/channels/` 与 locale 中关联 Azure/被删渠道的键）

对每个候选键运行 `findstr /s /m /c:"<key>" web\src\*.ts web\src\*.tsx`（排除 locales/ 与 static-keys.ts）：零引用且属于被删渠道 → 删除。

- [ ] **Step 2: 删除 7 个 locale 的键**

用一次性脚本（Node，从 `web/` 目录运行）删除 7 个 locale 文件中确认的键，例如：
```js
// scripts/remove-keys.mjs（临时脚本，用完删除）
const fs = require('fs');
const keys = ['Azure', 'Baidu', /* ...确认清单... */];
for (const lang of ['en','zh','zh-TW','fr','ja','ru','vi']) {
  const p = `src/i18n/locales/${lang}.json`;
  const data = JSON.parse(fs.readFileSync(p, 'utf8'));
  let changed = false;
  for (const k of keys) { if (k in data) { delete data[k]; changed = true; } }
  if (changed) fs.writeFileSync(p, JSON.stringify(data, null, 2) + '\n');
}
```
（保持文件格式：2 空格缩进 + 末尾换行，与现有文件一致）

- [ ] **Step 3: static-keys 同步**

`web/src/i18n/static-keys.ts` 检查并删除同样键（若存在）。

- [ ] **Step 4: 验证与提交**

```bash
bun run i18n:sync   # 确认无缺失键/无错误
bun run typecheck
```
```bash
git add -A && git commit --no-gpg-sign -m "chore(i18n): remove deleted channel keys from all locales (task 5)"
```

---

### Task 6: 全量验证与单次提交

**Files:**
- 全仓库改动

**Interfaces:**
- Consumes: Task 1-5 全部完成

- [ ] **Step 1: 后端全量验证**

```bash
go build ./...
go vet ./...
go test ./... -count=1
```
预期：仅 2 个预存不稳定测试失败（`service` channel_affinity 2 个）；TestUpstreamGetBody 抖动则重跑确认。

- [ ] **Step 2: relaykit 独立构建**

`cd relaykit && cmd /c "set GOWORK=off&&go build ./..."`
预期：成功（本计划未改 relaykit，确认无意外影响）。

- [ ] **Step 3: gofmt 与残留扫描**

```bash
gofmt -l .   # 排除 web/、.codegraph/；预存 controller/misc.go 除外，不得有新增
findstr /s /m /c:"ChannelTypeAzure" /c:"ChannelTypeAli" /c:"ChannelTypeDeepSeek" /c:"ChannelTypeMoonshot" /c:"ChannelSpecialBases" /c:"ChannelTypeVolcEngine" /c:"ChannelTypeZhipu" /c:"ChannelTypeTencent" /c:"ChannelTypeBaidu" /c:"ChannelType360" /c:"ChannelTypeLingYiWanWu" /c:"ChannelTypeMiniMax" /c:"ChannelTypeSiliconFlow" /c:"ChannelTypeMokaAI" /c:"ChannelTypeXunfei" /c:"ChannelTypeBaiduV2" /c:"APITypeMoonshot" /c:"APITypeMiniMax" *.go
```
预期：全部零命中（docs/ 与 web/ 另行扫描，孤儿键已在 Task 5 处理；`docs/openapi/relay.json` 若含示例文案属纯文档，记录 ledger 不阻塞）。

- [ ] **Step 4: codegraph 复查**

```bash
codegraph sync
codegraph query "ChannelTypeAzure" / "ChannelTypeAli" / "ChannelTypeDeepSeek" / "ChannelSpecialBases" / "APITypeMoonshot"
```
预期：零残留。

- [ ] **Step 5: 前端全量验证**

```bash
bun run typecheck && bun run test && bun run build
```
预期：全过（预存 lint error 仅在未改动文件，不阻塞）。

- [ ] **Step 6: 单次提交**（控制器执行）

控制器执行：
```bash
git reset --soft 177cf137   # spec 提交（docs: add design spec for removing 16 more channel types）
git add -A
git commit --no-gpg-sign -m "refactor(channel): remove 16 more channel types (Azure and 15 Chinese providers)

- Remove adapters: Azure(inline), Baidu, Zhipu, ZhipuV4, Ali, Xunfei,
  360, Tencent, Moonshot, LingYiWanWu, MiniMax, SiliconFlow, DeepSeek,
  MokaAI, VolcEngine, BaiduV2
- Remove ChannelSpecialBases coding-plan mappings and stream support,
  usage, billing special cases; channel type values keep original numbering
- Remove frontend options, form fields, vendor rules, ratios and i18n keys
- Keep OpenAI, Anthropic, Gemini, Custom, Codex, AdvancedCustom, Sub2API,
  NewAPI and the Dummy sentinel"
```

- [ ] **Step 7: 收尾报告**

汇报用户：改动统计、验证结果、预存不稳定测试说明、保留项确认、遗留 ledger（文档残留等）。
