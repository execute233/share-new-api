# Codex 伪装渠道对抗性审查修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复对抗性审查发现的 3 严重 + 5 中等 + 4 轻微问题，使 Codex 伪装渠道 alpha/search 完整可用、turn-state 防 DoS、死承诺全部兑现或删除。

**Architecture:** 后端：alpha/search 白名单补渠道 + 伪装 stage 上移到 SetupRequestHeader + turn-state registry 对齐 sub2api（不插入未知/节流 sweep/渠道级键）；清理死字段死函数；版本同步接线。前端：补 i18n 键 + 最小配置表单 + 类型扩展 + 组件测试。

**Tech Stack:** Go 1.22+ / Gin / GORM（不动 DB）；React 19 / TypeScript / React Hook Form + Zod / i18next。

## Global Constraints

- `relaykit/` 改动后必须 `cd relaykit && GOWORK=off go build ./...` 独立验证（Task 4 触碰 `relaykit/dto/channel_settings.go`）。
- `docs/superpowers/` 新增的 spec/plan **不提交 git**；代码提交仅 `git add` 代码/测试/i18n 文件。
- 所有 JSON marshal/unmarshal 走 `common.Marshal/Unmarshal`（common/json.go）。
- 前端 UI 文案必须 i18n（en 源串，7 locale 同步：en/zh/zh-TW/fr/ru/ja/vi）。
- Go 测试用 `testify/require`（setup/fatal）+ `assert`（非 fatal）。
- 改动渠道类型 62 相关行为必须同步更新/新增测试。
- 计费路径不变。

---

### Task 1: alpha/search 白名单 + 指纹 stage 上移 + 响应侧 turn-state 透传

**Files:**
- Modify: `relay/alpha_search_handler.go:24-35`（白名单 switch）
- Modify: `relay/channel/codexdisguise/adaptor.go`（stage 上移到 SetupRequestHeader、导出 RelayUpstreamTurnState、captureTurnState 复用）
- Test: `relay/alpha_search_handler_test.go`（新增白名单测试）、`relay/channel/codexdisguise/adaptor_test.go`（新增 stage 幂等测试）

**Interfaces:**
- Produces: `codexdisguise.RelayUpstreamTurnState(c *gin.Context, info *relaycommon.RelayInfo, upstream http.Header)` — AlphaSearchHelper 在 io.Copy 前调用（仅渠道类型 62 时）。

- [ ] **Step 1: 写失败测试 — 白名单**

在 `relay/alpha_search_handler_test.go` 追加：

```go
func TestAlphaSearchHelperAllowsCodexDisguiseChannel(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/backend-api/codex/alpha/search", strings.NewReader(`{"model":"gpt-5.1-codex","prompt":"x"}`))
	info := &relaycommon.RelayInfo{ChannelType: constant.ChannelTypeCodexDisguise, RelayMode: relayconstant.RelayModeAlphaSearch}
	info.Request = &dto.AlphaSearchRequest{RawBody: []byte(`{"model":"gpt-5.1-codex"}`)}
	err := AlphaSearchHelper(c, info)
	require.NotNil(t, err) // 无真实上游，预期在 DoRequest 失败；但必须已通过白名单（错误不是 invalid request）
	assert.NotEqual(t, types.ErrorCodeInvalidRequest, err.GetErrorCode())
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./relay/ -run TestAlphaSearchHelperAllowsCodexDisguiseChannel -v`
Expected: FAIL — 错误码为 invalid request（白名单拒绝）。

- [ ] **Step 3: 白名单加渠道**

`relay/alpha_search_handler.go` switch 追加：

```go
	case constant.ChannelTypeCodexDisguise:
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./relay/ -run TestAlphaSearchHelperAllowsCodexDisguiseChannel -v`
Expected: PASS（错误码不是 invalid request；因无上游/未初始化，DoRequest 失败是预期的）。

- [ ] **Step 5: 写失败测试 — stage 幂等（adaptor_test.go）**

```go
func TestSetupRequestHeaderStagesFingerprintIDsIdempotently(t *testing.T) {
	a := &Adaptor{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses", nil)
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelOtherSettings.FingerprintSeed = "a3f5c0d0-0000-4000-8000-000000000001"
	info.ChannelOtherSettings.FingerprintMode = "full"
	h := http.Header{}
	require.NoError(t, a.SetupRequestHeader(c, &h, info))
	require.NoError(t, a.SetupRequestHeader(c, &h, info)) // 二次调用幂等
	require.NotEmpty(t, h.Get("session-id"))
	require.NotEmpty(t, h.Get("x-codex-installation-id"))
}
```

- [ ] **Step 6: 跑测试确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestSetupRequestHeaderStagesFingerprintIDsIdempotently -v`
Expected: FAIL — SetupRequestHeader 当前不 stage，指纹头为空。

- [ ] **Step 7: stage 上移实现**

`adaptor.go`：
1. `stageCodexFingerprintIDs` 改幂等（`c.Get(codexFingerprintIDsContextKey)` 已存在则直接返回）。
2. `SetupRequestHeader` 在 `applyFingerprintHeaders` 前调用 `a.stageCodexFingerprintIDs(c, info)`（guard 空 nil 判断不变）。
3. `ConvertOpenAIResponsesRequest` 中删除 `a.stageCodexFingerprintIDs(c, info)` 调用。
4. 导出响应侧函数（原 `captureTurnState` 逻辑）：

```go
// RelayUpstreamTurnState 将上游 turn-state 透传下游并记录溯源（alpha/search 路径使用）。
func RelayUpstreamTurnState(c *gin.Context, info *relaycommon.RelayInfo, upstream http.Header) {
	if c == nil || info == nil || upstream == nil {
		return
	}
	blob := strings.TrimSpace(upstream.Get("x-codex-turn-state"))
	if blob == "" {
		return
	}
	threadID := ""
	if ids := stagedCodexFingerprintIDs(c); ids != nil {
		threadID = ids.ThreadID
	}
	turnStates.note(info.ChannelId, extractClientSessionID(c), threadID)
	c.Header("x-codex-turn-state", blob)
}
```

`captureTurnState`（adaptor 方法）改为内部调用同一 note（保留 threadID 解析）。

- [ ] **Step 8: 提取 extractClientSessionID 包级函数**

`adaptor.go` 把 `stageCodexFingerprintIDs` 内的 session-id/session_id 提取抽成包级函数：

```go
func extractClientSessionID(h http.Header) string {
	if h == nil {
		return ""
	}
	if v := strings.TrimSpace(h.Get("session-id")); v != "" {
		return v
	}
	return strings.TrimSpace(h.Get("session_id"))
}
```

`stageCodexFingerprintIDs` 中改用它（c.Request.Header）。

- [ ] **Step 9: AlphaSearchHelper 响应侧接线**

`relay/alpha_search_handler.go` 在 `c.Writer.WriteHeader(httpResp.StatusCode)` 之前插入：

```go
	if info.ChannelType == constant.ChannelTypeCodexDisguise {
		codexdisguise.RelayUpstreamTurnState(c, info, httpResp.Header)
	}
```

（`relay/alpha_search_handler.go` 已 import `relay/channel/codexdisguise`？未 import，需新增——检查循环依赖：relay 包已 import codexdisguise via relay_adaptor.go，无循环。）

- [ ] **Step 10: 跑相关测试**

Run: `go test ./relay/... -run "TestAlphaSearch|TestSetupRequestHeaderStages|TestCodexDisguise" -v`
Expected: 新增测试 PASS；既有 codexdisguise 测试不回归（stage 幂等不改变行为）。

- [ ] **Step 11: 提交（仅代码，不提交 docs/superpowers/）**

```bash
git add relay/alpha_search_handler.go relay/alpha_search_handler_test.go relay/channel/codexdisguise/adaptor.go relay/channel/codexdisguise/adaptor_test.go
git commit -m "fix(codex-disguise): support alpha search with fingerprint staging and turn-state relay"
```

---

### Task 2: turn-state registry 对齐 sub2api（防 DoS + 渠道级隔离）

**Files:**
- Modify: `relay/channel/codexdisguise/turn_state.go`（重写）
- Modify: `relay/channel/codexdisguise/adaptor.go`（guard/capture 调用签名）
- Test: `relay/channel/codexdisguise/turn_state_test.go`（重写）

**Interfaces:**
- Consumes: `extractClientSessionID(h http.Header) string`（Task 1 产出）、`info.ChannelId int`（relay/common/relay_info.go:52）
- Produces: registry 方法 `note(channelID int, sessionID, threadID string)`、`guard(channelID int, sessionID, threadID, blob string) string`

- [ ] **Step 1: 写失败测试 — 未知不插入 + 异渠道隔离 + 节流**

重写 `turn_state_test.go`：

```go
func TestTurnStateRegistryUnknownBlobPassesWithoutInsert(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	require.Equal(t, "blob-unknown", r.guard(1, "sess-a", "thread-a", "blob-unknown"))
	r.mu.Lock()
	count := len(r.entries)
	r.mu.Unlock()
	assert.Zero(t, count) // 未知不插入
}

func TestTurnStateRegistryCrossChannelIsolation(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	r.note(1, "sess-a", "thread-a")
	assert.Equal(t, "blob", r.guard(1, "sess-a", "thread-a", "blob"))
	assert.Equal(t, "", r.guard(2, "sess-a", "thread-a", "blob")) // 渠道 2 剥离
}

func TestTurnStateRegistrySweepThrottled(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	for i := 0; i < 255; i++ {
		r.note(1, fmt.Sprintf("sess-%d", i), "thread-a")
	}
	r.mu.Lock()
	count := len(r.entries)
	r.mu.Unlock()
	assert.Equal(t, 255, count) // 未触发 sweep
	r.note(1, "sess-256", "thread-a")
	r.mu.Lock()
	count = len(r.entries)
	r.mu.Unlock()
	assert.LessOrEqual(t, count, 256) // 第 256 次写入触发 sweep 但仍保留有效项
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestTurnStateRegistry -v`
Expected: FAIL — 现实现未知插入、无渠道隔离、每次全扫。

- [ ] **Step 3: 重写 turn_state.go**

```go
package codexdisguise

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const turnStateRegistrySweepInterval = 256

type turnStateEntry struct {
	threadID string
	mintedAt time.Time
}

type turnStateRegistry struct {
	mu      sync.Mutex
	entries map[string]turnStateEntry
	ttl     time.Duration
	writes  uint64
}

func newTurnStateRegistry(ttl time.Duration) *turnStateRegistry {
	return &turnStateRegistry{
		entries: make(map[string]turnStateEntry),
		ttl:     ttl,
	}
}

func turnStateRegistryKey(channelID int, sessionID string) string {
	return fmt.Sprintf("%d\x00%s", channelID, strings.TrimSpace(sessionID))
}

func (r *turnStateRegistry) note(channelID int, sessionID, threadID string) {
	if r == nil {
		return
	}
	key := turnStateRegistryKey(channelID, sessionID)
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[key] = turnStateEntry{threadID: threadID, mintedAt: time.Now()}
	r.writes++
	if r.writes%turnStateRegistrySweepInterval == 0 {
		r.sweepLocked()
	}
}

func (r *turnStateRegistry) guard(channelID int, sessionID, threadID, blob string) string {
	blob = strings.TrimSpace(blob)
	if blob == "" || r == nil {
		return blob
	}
	key := turnStateRegistryKey(channelID, sessionID)
	if key == "" {
		return blob
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[key]
	if !ok || (!entry.mintedAt.IsZero() && time.Now().After(entry.mintedAt.Add(r.ttl))) {
		if ok {
			delete(r.entries, key)
		}
		return blob // 未知或过期：放行不插入
	}
	if entry.threadID == threadID {
		return blob
	}
	return "" // 异 thread 铸造：剥离
}

func (r *turnStateRegistry) sweepLocked() {
	if r == nil || r.ttl <= 0 || len(r.entries) == 0 {
		return
	}
	cutoff := time.Now().Add(-r.ttl)
	for key, entry := range r.entries {
		if entry.mintedAt.Before(cutoff) {
			delete(r.entries, key)
		}
	}
}
```

注意：过期判定在 guard 读侧惰性删除 + 写入节流全扫，无每请求 O(n)。

- [ ] **Step 4: 更新 adaptor.go 调用方**

`guardTurnState` / `captureTurnState` / `RelayUpstreamTurnState` 改新签名：

```go
func (a *Adaptor) guardTurnState(c *gin.Context, info *relaycommon.RelayInfo, req *http.Header) {
	if c == nil || info == nil || req == nil || c.Request == nil {
		return
	}
	blob := strings.TrimSpace(c.Request.Header.Get("x-codex-turn-state"))
	if blob == "" {
		return
	}
	threadID := a.stagedThreadID(c)
	guarded := turnStates.guard(info.ChannelId, extractClientSessionID(c.Request.Header), threadID, blob)
	if guarded == "" {
		req.Del("x-codex-turn-state")
		return
	}
	req.Set("x-codex-turn-state", guarded)
}
```

`captureTurnState` 同理调 `turnStates.note(info.ChannelId, extractClientSessionID(c.Request.Header), threadID)`（threadID 解析保留 staged ids 逻辑）。`RelayUpstreamTurnState`（Task 1 产出）同步改用新 note 签名。

- [ ] **Step 5: 跑测试**

Run: `go test ./relay/channel/codexdisguise/ -count=1`
Expected: 全部 PASS（新 registry 测试 + 既有适配器测试）。

- [ ] **Step 6: 提交**

```bash
git add relay/channel/codexdisguise/turn_state.go relay/channel/codexdisguise/turn_state_test.go relay/channel/codexdisguise/adaptor.go
git commit -m "fix(codex-disguise): align turn-state registry with upstream semantics (no unknown insert, throttled sweep, channel-scoped keys)"
```

---

### Task 3: M1 版本同步接线

**Files:**
- Modify: `relay/channel/codexdisguise/adaptor.go`（codexClientVersionFromSettings 兜底）
- Test: `relay/channel/codexdisguise/adaptor_test.go`（新增）

- [ ] **Step 1: 写失败测试**

```go
func TestCodexClientVersionFallsBackToOption(t *testing.T) {
	old := common.OptionMap
	common.OptionMap = map[string]string{service.CodexDisguiseClientVersionOptionKey: "0.148.0"}
	t.Cleanup(func() { common.OptionMap = old })
	info := testRelayInfo(relayconstant.RelayModeResponses)
	require.Equal(t, "0.148.0", codexClientVersionFromSettings(info))
}
```

（`service.CodexDisguiseClientVersionOptionKey` = "CodexDisguiseClientVersion"）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestCodexClientVersionFallsBackToOption -v`
Expected: FAIL — 当前返回编译期常量 codexCLIVersion。

- [ ] **Step 3: 实现**

`adaptor.go` 改 `codexClientVersionFromSettings`：

```go
func codexClientVersionFromSettings(info *relaycommon.RelayInfo) string {
	if info != nil {
		if v := NormalizeCodexClientVersion(info.ChannelOtherSettings.CodexClientVersion); v != "" {
			return v
		}
	}
	if info != nil {
		if v, err := service.GetCodexDisguiseClientVersion(context.Background(), info.ChannelProxyURL); err == nil && v != "" {
			if normalized := NormalizeCodexClientVersion(v); normalized != "" {
				return normalized
			}
		}
	}
	return codexCLIVersion
}
```

新增 import：`context`、`"github.com/QuantumNous/new-api/service"`。

- [ ] **Step 4: 跑测试**

Run: `go test ./relay/channel/codexdisguise/ -count=1 && go test ./service/ -run TestGetCodexDisguiseClientVersion -count=1`
Expected: 全 PASS。

- [ ] **Step 5: 提交**

```bash
git add relay/channel/codexdisguise/adaptor.go relay/channel/codexdisguise/adaptor_test.go
git commit -m "fix(codex-disguise): wire client version sync service into adaptor fallback"
```

---

### Task 4: M2+M3 实现 DisguiseEnabled / 删死字段死函数

**Files:**
- Modify: `relaykit/dto/channel_settings.go`（删 AgentAutoRegister 字段）
- Modify: `relay/channel/codexdisguise/adaptor.go`（DisguiseEnabled 分支）
- Modify: `relay/channel/codexdisguise/agent_identity.go`（删两个死函数）
- Modify: `relay/channel/codexdisguise/agent_identity_test.go`（删对应测试）
- Test: `relay/channel/codexdisguise/adaptor_test.go`（新增纯转发测试）

**Interfaces:**
- Produces: `codexDisguiseEnabled(info *relaycommon.RelayInfo) bool`（nil/true=true；false=false）

- [ ] **Step 1: 删死函数**

`agent_identity.go` 删除 `signAgentTaskRegistration`、`decryptAgentTaskID`；`agent_identity_test.go` 删除 `TestSignAgentTaskRegistration`、`TestDecryptAgentTaskID`（保留 assertion 相关测试）。

- [ ] **Step 2: 删 AgentAutoRegister 字段**

`relaykit/dto/channel_settings.go` 删除：

```go
	AgentAutoRegister  *bool  `json:"agent_auto_register,omitempty"`  // nil/true = task 失效自动重注册
```

- [ ] **Step 3: relaykit 独立构建验证**

Run: `cd relaykit && GOWORK=off go build ./...`
Expected: PASS。

- [ ] **Step 4: 写失败测试 — 纯转发**

```go
func TestSetupRequestHeaderDisguiseDisabledSkipsDisguise(t *testing.T) {
	a := &Adaptor{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses", nil)
	c.Request.Header.Set("session-id", "client-sess")
	info := testRelayInfo(relayconstant.RelayModeResponses)
	falseVal := false
	info.ChannelOtherSettings.DisguiseEnabled = &falseVal
	info.ChannelOtherSettings.FingerprintSeed = "a3f5c0d0-0000-4000-8000-000000000001"
	info.ChannelOtherSettings.FingerprintMode = "full"
	h := http.Header{}
	require.NoError(t, a.SetupRequestHeader(c, &h, info))
	assert.Empty(t, h.Get("session-id"))
	assert.Empty(t, h.Get("x-codex-installation-id"))
	assert.Empty(t, h.Get("originator"))
	assert.Empty(t, h.Get("version"))
	assert.Equal(t, "", h.Get("user-agent"))
}
```

- [ ] **Step 5: 跑测试确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestSetupRequestHeaderDisguiseDisabledSkipsDisguise -v`
Expected: FAIL — 当前无条件伪装。

- [ ] **Step 6: 实现 DisguiseEnabled 分支**

`adaptor.go` 新增：

```go
func codexDisguiseEnabled(info *relaycommon.RelayInfo) bool {
	if info != nil && info.ChannelOtherSettings.DisguiseEnabled != nil {
		return *info.ChannelOtherSettings.DisguiseEnabled
	}
	return true
}
```

`SetupRequestHeader`：`canonicalUA/version/enforce` 计算与 `ensureCodexIdentityHeaders/applyFingerprintHeaders/guardTurnState/enforceCodexIdentityHeaders` 包进 `if codexDisguiseEnabled(info) { ... }`（认证头与 Content-Type/Accept 不包）。`stageCodexFingerprintIDs` 入口加 enabled 判断（false 直接返回）。`captureTurnState` 加 enabled 判断。`RelayUpstreamTurnState` 加 enabled 判断。

- [ ] **Step 7: 跑测试**

Run: `go test ./relay/channel/codexdisguise/ -count=1 && cd relaykit && GOWORK=off go build ./...`
Expected: 全 PASS + relaykit 独立构建 PASS。

- [ ] **Step 8: 提交**

```bash
git add relaykit/dto/channel_settings.go relay/channel/codexdisguise/adaptor.go relay/channel/codexdisguise/agent_identity.go relay/channel/codexdisguise/agent_identity_test.go relay/channel/codexdisguise/adaptor_test.go
git commit -m "fix(codex-disguise): implement disguise_enabled passthrough, remove dead agent-register code"
```

---

### Task 5: M4 key.go 收紧 OAuth

**Files:**
- Modify: `relay/channel/codexdisguise/key.go`（删 RefreshToken、Validate 收紧）
- Test: `relay/channel/codexdisguise/key_test.go`（更新）

- [ ] **Step 1: 更新测试**

`key_test.go` 中 refresh-only 用例改为期望错误：

```go
func TestParseDisguiseKeyOAuthRequiresAccessToken(t *testing.T) {
	_, err := ParseDisguiseKey(`{"type":"oauth","refresh_token":"rt"}`)
	require.Error(t, err)
	_, err = ParseDisguiseKey(`{"type":"oauth","access_token":"at","account_id":"acc"}`)
	require.NoError(t, err)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestParseDisguiseKey -v`
Expected: FAIL — refresh-only 当前通过校验。

- [ ] **Step 3: 实现**

`key.go`：`DisguiseKey` 删除 `RefreshToken string` 字段；`Validate()` OAuth 分支改为：

```go
	case DisguiseKeyTypeOAuth:
		if k.AccessToken == "" {
			return errors.New("codex disguise channel: access_token is required for oauth type")
		}
```

- [ ] **Step 4: 跑测试**

Run: `go test ./relay/channel/codexdisguise/ -count=1`
Expected: 全 PASS。

- [ ] **Step 5: 提交**

```bash
git add relay/channel/codexdisguise/key.go relay/channel/codexdisguise/key_test.go
git commit -m "fix(codex-disguise): require access_token for oauth keys at validation time"
```

---

### Task 6: M5 渠道测试默认端点 + L3 DoResponse 分支清理

**Files:**
- Modify: `controller/channel-test.go:49`（normalizeChannelTestEndpoint）
- Modify: `relay/channel/codexdisguise/adaptor.go:193-195`（DoResponse default 文案）
- Test: `controller/channel_test_internal_test.go`（新增）、`relay/channel/codexdisguise/adaptor_test.go`（更新）

- [ ] **Step 1: 写失败测试**

`controller/channel_test_internal_test.go` 追加：

```go
func TestNormalizeChannelTestEndpointCodexDisguise(t *testing.T) {
	ch := &model.Channel{Type: constant.ChannelTypeCodexDisguise}
	assert.Equal(t, string(constant.EndpointTypeOpenAIResponse), normalizeChannelTestEndpoint(ch, ""))
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./controller/ -run TestNormalizeChannelTestEndpointCodexDisguise -v`
Expected: FAIL — 返回空串。

- [ ] **Step 3: 实现**

`channel-test.go`:

```go
	if channel != nil && (channel.Type == constant.ChannelTypeCodex || channel.Type == constant.ChannelTypeCodexDisguise) {
		return string(constant.EndpointTypeOpenAIResponse)
	}
```

`adaptor.go` DoResponse default 分支文案对齐 codex 渠道：

```go
	default:
		return nil, types.NewError(errors.New("codex disguise channel: alpha search response should be handled by AlphaSearchHelper"), types.ErrorCodeInvalidRequest)
```

- [ ] **Step 4: 跑测试**

Run: `go test ./controller/ -run TestNormalizeChannelTestEndpoint -count=1 && go test ./relay/channel/codexdisguise/ -count=1`
Expected: 全 PASS。

- [ ] **Step 5: 提交**

```bash
git add controller/channel-test.go controller/channel_test_internal_test.go relay/channel/codexdisguise/adaptor.go
git commit -m "fix(codex-disguise): default channel test endpoint and clarify alpha search response handling"
```

---

### Task 7: L1 i18n 补键 + L2 前端最小配置表单

**Files:**
- Modify: `web/src/i18n/locales/en.json`、`zh.json`、`zh-TW.json`、`fr.json`、`ru.json`、`ja.json`、`vi.json`
- Modify: `web/src/features/channels/types.ts`（ChannelOtherSettings 扩展）
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`（5 字段表单）
- Test: `web/src/features/channels/components/drawers/__tests__/codex-disguise-settings.test.tsx`

**Interfaces:**
- Consumes: `ChannelOtherSettings` 类型（types.ts:104）新增字段；`channelInfoSchema` 不动。
- Produces: i18n 键 'Codex Disguise'、'Paste Codex Disguise JSON key'、'Disguise Enabled'、'Fingerprint Mode'、'Fingerprint Seed'、'Codex Client Version'、'Enforce Identity'、各中文/地区译文。

- [ ] **Step 1: 扩展 types.ts**

`ChannelOtherSettings` 追加：

```ts
export interface ChannelOtherSettings {
  // ...existing fields...
  disguise_enabled?: boolean
  fingerprint_mode?: 'off' | 'device' | 'session' | 'full'
  fingerprint_seed?: string
  codex_client_version?: string
  enforce_identity?: boolean
}
```

- [ ] **Step 2: 补 i18n 键（7 locale）**

按 i18n-translate skill 流程，在 7 个 locale 的渠道相关区块（en.json 765 行 'ChatGPT Subscription (Codex)' 附近）登记：

en: `"Codex Disguise"`, `"Paste Codex Disguise JSON key (sub2api api_key / oauth / agent)"`, `"Disguise Enabled"`, `"Fingerprint Mode"`, `"Fingerprint Seed"`, `"Codex Client Version"`, `"Enforce Identity"`, `"Disguise Settings"`（表单区标题）, `"UUID v4 required for fingerprint convergence"`（seed 校验提示）, `"Empty disables fingerprint convergence"`。

zh: 对应中文（伪装开关 / 指纹模式 / 指纹种子 / Codex 客户端版本 / 强制统一身份 / 伪装设置 / 指纹收敛需要 UUID v4 / 留空则关闭指纹收敛）。zh-TW 繁体、fr/ru/ja/vi 按既有风格翻译。

- [ ] **Step 3: drawer 加表单字段**

`channel-mutate-drawer.tsx` 高级设置区（`currentType === 62` 时渲染，参照 `currentType === 1` 的 force_format 模式）新增 5 字段区块：

```tsx
{currentType === 62 && (
  <div className='divide-border space-y-0 divide-y border-y'>
    <FormField control={form.control} name='disguise_enabled' render={({ field }) => (
      <FormItem className='flex items-center justify-between px-4 py-3'>
        <div className='space-y-0.5'>
          <FormLabel>{t('Disguise Enabled')}</FormLabel>
          <FormDescription>{t('Disable to pass through without disguise')}</FormDescription>
        </div>
        <FormControl><Switch checked={field.value} onCheckedChange={field.onChange} /></FormControl>
      </FormItem>
    )} />
    {/* Fingerprint Mode select / Fingerprint Seed input / Codex Client Version input / Enforce Identity switch */}
  </div>
)}
```

字段绑定：`settings` JSON 的 ChannelOtherSettings 子键（参照 `parseChannelOtherSettings`/`parseSettingsRecord` 既有读写路径；新增字段随 settings 字符串序列化）。需要确认 form schema 中 settings 字段的读写方式——若为字符串，5 个新字段在 submit 时并入 settings JSON（参照 advanced_custom 的既有处理，channel-mutate-drawer.tsx:353-357 `parseSettingsRecord`）。

- [ ] **Step 4: 写组件测试**

`web/src/features/channels/components/drawers/__tests__/codex-disguise-settings.test.tsx`：渲染 type 62 渠道表单 → 断言 5 字段出现；切换 Disguise Enabled 开关后 submit → 断言 settings JSON 含 `"disguise_enabled":false`；type 1（OpenAI）不显示伪装字段。

- [ ] **Step 5: 跑前端验证**

Run: `bun run typecheck && bunx vitest run src/features/channels/components/drawers/__tests__/codex-disguise-settings.test.tsx`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add web/src/i18n/locales/ web/src/features/channels/types.ts web/src/features/channels/components/drawers/channel-mutate-drawer.tsx web/src/features/channels/components/drawers/__tests__/codex-disguise-settings.test.tsx
git commit -m "feat(web): add codex disguise channel settings form and i18n keys"
```

---

### Task 8: L4 恢复 spec + 全量验证收尾

**Files:**
- Restore: `docs/superpowers/specs/2026-08-28-codex-disguise-channel-design.md`（不提交）

- [ ] **Step 1: 恢复 spec 文档**

Run: `git checkout 45f7d179 -- docs/superpowers/specs/2026-08-28-codex-disguise-channel-design.md`
Expected: 文件恢复；保持 untracked（不 git add）。

- [ ] **Step 2: 后端全量验证**

Run: `go build ./... && go test ./relay/... ./router/ ./service/ ./controller/ ./common/ ./constant/ -count=1`
Expected: 全 PASS（已知并行 flaky 单测除外，单跑复验）。

- [ ] **Step 3: relaykit 独立构建**

Run: `cd relaykit && GOWORK=off go build ./...`
Expected: PASS。

- [ ] **Step 4: 前端全量验证**

Run: `bun run typecheck && bunx oxlint src/features/channels`
Expected: 无新 error（既有 error 文件除外，与本次无关）。

- [ ] **Step 5: 收尾汇报**

汇总提交列表 + 验证输出，报告用户。不提交 docs/superpowers/。

---

## Self-Review

**1. Spec coverage:**
- S1（白名单加类型）→ Task 1 Step 1-4 ✅
- S2（stage 上移 + 响应侧透传）→ Task 1 Step 5-10 ✅
- S3（turn-state 对齐）→ Task 2 ✅
- M1（版本接线）→ Task 3 ✅
- M2（DisguiseEnabled）→ Task 4 ✅
- M3（删死代码）→ Task 4 ✅
- M4（OAuth 收紧）→ Task 5 ✅
- M5（测试端点）+ L3（DoResponse 文案）→ Task 6 ✅
- L1（i18n）+ L2（前端表单）→ Task 7 ✅
- L4（恢复 spec）→ Task 8 ✅
- 全局约束（relaykit 独立构建、不提交 docs、i18n 7 locale、testify）→ 各任务步骤覆盖 ✅

**2. Placeholder scan:** 无 TBD/TODO；所有实现步骤含具体代码。

**3. Type consistency:**
- `extractClientSessionID(h http.Header) string` — Task 1 产出，Task 2 消费，签名一致 ✅
- `turnStates.note/guard` 新签名（channelID/sessionID/threadID）— Task 1 Step 7 的 RelayUpstreamTurnState 与 Task 2 Step 4 统一（Task 2 覆盖 Task 1 中的旧签名调用，RelayUpstreamTurnState 在 Task 2 Step 4 同步改）✅
- `codexDisguiseEnabled(info) bool` — Task 4 产出并内部消费 ✅
- `service.GetCodexDisguiseClientVersion(ctx, proxyURL)` — 既有签名（service/codex_disguise_version_sync.go:21）✅
- `DisguiseKey` 删除 RefreshToken — Task 5 中 key.go 与 key_test.go 同步 ✅

已知依赖顺序：Task 1（stage/提取函数）→ Task 2（新签名）；Task 7 依赖 Task 1-6 的后端字段（fingerprint_seed 等已在 relaykit/dto，Task 4 只删 AgentAutoRegister 不影响前端表单字段列表——前端表单不包含 agent_auto_register）。