# Codex Disguise 渠道（Codex伪装）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在主模块新增 `ChannelTypeCodexDisguise`（Codex伪装）渠道：参照 sub2api 的伪装能力（身份三元组、指纹收敛、turn-state、Agent Identity），对外暴露标准接口与 codex 原生路径，将下游多个 codex 客户端合并为统一会话，出站接入上游 sub2api 实例。

**Architecture:** 新建 `relay/channel/codexdisguise/` 适配器包，移植 sub2api 的伪装引擎（身份/指纹/turn-state/Agent Identity 为纯函数，无 DB 依赖）；渠道 Key 为多格式 JSON（sub2api api_key / OAuth auth / Agent Identity）；出站走 sub2api 的 `/backend-api/codex/*` 原生路径；新增 `/backend-api/codex` 入站路由组；复用主模块已有 `service.RefreshCodexOAuthTokenWithProxy` 与 `service.GetLatestCodexClientVersion`。

**Tech Stack:** Go 1.22+、Gin、GORM、crypto/ed25519、golang.org/x/crypto（nacl/box、curve25519）、google/uuid、github.com/stretchr/testify（require/assert）

## Global Constraints

- 设计文档：`docs/superpowers/specs/2026-08-28-codex-disguise-channel-design.md`（已确认，本计划按 spec 实现）
- 渠道类型常量：`ChannelTypeCodexDisguise = 62`；`ChannelTypeDummy` 移为 63
- 身份三元组必须自洽：originator 与 UA 首段配套；version ≥ 0.144.0 且等于 UA 版本段；否则上游 404（issue #3901）
- 默认 originator：`codex-tui`；默认版本：`0.146.0`（编译期兜底）
- UA 形态：`codex-tui/{version} (Ubuntu 22.4.0; x86_64) xterm-256color`
- 指纹默认模式：`session`（1 设备 1 会话 N 线程）；off 为显式 opt-in 关闭
- Key 三格式：`{"type":"sub2api","api_key":...}` / `{"type":"oauth","access_token":...,"refresh_token":...,"account_id":...}` / `{"type":"agent","agent_private_key":...,"agent_runtime_id":...,"agent_task_id":...}`
- 上游 sub2api **不改动**；账号由 sub2api 管理员配置
- 仅支持 Responses / ResponsesCompact / AlphaSearch 三种 RelayMode，其余端点一律拒绝
- 请求体强制：`store=false`，删除 `max_output_tokens`/`temperature`/`frequency_penalty`/`presence_penalty`，`instructions` 缺省注入 `""`
- 强制 `Content-Type: application/json`；流式 `Accept: text/event-stream`；compact `Accept: application/json`
- 多 Key 渠道不支持（`channel_info.is_multi_key` 拒绝）
- 本期不做：WS/realtime、wham 面板、sub2api 模块改动
- 所有 JSON 操作走 `common.Marshal/Unmarshal`，不直接调用 `encoding/json`
- 与 sub2api 的伪装算法保持语义一致，但命名空间前缀用 `new-api:` 区分（避免与 sub2api 派生值冲突）
- relaykit 模块不得被本计划涉及（`relay/channel/codexdisguise` 属于主模块）
- 完成后必须验证：`go build ./...`、`go test ./relay/channel/codexdisguise/...`、`cd relaykit && GOWORK=off go build ./...`

---

### Task 1: 渠道类型常量与注册

**Files:**
- Modify: `constant/channel.go`
- Modify: `relay/relay_adaptor.go`
- Modify: `web/src/features/channels/constants.ts`
- Test: `relay/channel/codexdisguise/constants_test.go`（随 Task 7 建包，本任务仅编译验证）

**Interfaces:**
- Produces: `constant.ChannelTypeCodexDisguise = 62`；`constant.ChannelTypeDummy = 63`；`ChannelBaseURLs` 索引 62 存在（默认 `""`）；`ChannelTypeNames[62] = "Codex Disguise"`

- [ ] **Step 1: 修改常量文件**

`constant/channel.go`（channel.go 常量区）：

```go
const (
	ChannelTypeUnknown        = 0
	ChannelTypeOpenAI         = 1
	ChannelTypeCustom         = 8
	ChannelTypeAnthropic      = 14
	ChannelTypeGemini         = 24
	ChannelTypeCodex          = 57
	ChannelTypeAdvancedCustom = 58
	ChannelTypeSub2API        = 59
	ChannelTypeNewAPI         = 60
	ChannelTypeCodexDisguise  = 62 // Codex 伪装渠道（多下游收敛 → sub2api 上游）
	ChannelTypeDummy          = 63 // this one is only for count, do not add any channel after this
)
```

`ChannelBaseURLs` 数组：索引 62 追加 `""`（伪装渠道默认空，渠道 BaseURL 必填指向 sub2api 实例），使数组长度达到 64（0..63）。

`ChannelTypeNames` map 追加：

```go
	ChannelTypeCodexDisguise: "Codex Disguise",
```

- [ ] **Step 2: 注册适配器**

`relay/relay_adaptor.go` 的 `GetAdaptor` switch 追加：

```go
	case constant.ChannelTypeCodexDisguise:
		return &codexdisguise.Adaptor{}
```

import 追加：`"github.com/QuantumNous/new-api/relay/channel/codexdisguise"`。

- [ ] **Step 3: 前端常量补位**

`web/src/features/channels/constants.ts`：

- `CHANNEL_TYPES` 追加 `62: 'Codex Disguise'`
- `CHANNEL_TYPE_DISPLAY_ORDER` 追加 `62`
- `TYPE_TO_KEY_PROMPT` 追加 `62: 'Paste Codex Disguise JSON key (sub2api api_key / oauth / agent)'`
- `getChannelTypeIcon`（`web/src/features/channels/lib/channel-utils.ts`）追加 `62: 'Codex'`

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 成功（codexdisguise 包尚未创建，需先创建最小包骨架；若报错则先执行 Task 2 Step 1 创建包后再回来验证）

- [ ] **Step 5: Commit**

```bash
git add constant/channel.go relay/relay_adaptor.go web/src/features/channels/constants.ts web/src/features/channels/lib/channel-utils.ts
git commit -m "feat(codex-disguise): register ChannelTypeCodexDisguise constant and adaptor"
```

---

### Task 2: Key 解析（key.go）

**Files:**
- Create: `relay/channel/codexdisguise/key.go`
- Create: `relay/channel/codexdisguise/key_test.go`

**Interfaces:**
- Produces:
  - `type DisguiseKeyType string`，常量 `DisguiseKeyTypeSub2API = "sub2api"`、`DisguiseKeyTypeOAuth = "oauth"`、`DisguiseKeyTypeAgent = "agent"`
  - `type DisguiseKey struct { Type DisguiseKeyType `json:"type"`; APIKey string `json:"api_key,omitempty"`; AccessToken string `json:"access_token,omitempty"`; RefreshToken string `json:"refresh_token,omitempty"`; AccountID string `json:"account_id,omitempty"`; AgentPrivateKey string `json:"agent_private_key,omitempty"`; AgentRuntimeID string `json:"agent_runtime_id,omitempty"`; AgentTaskID string `json:"agent_task_id,omitempty"` }`
  - `func ParseDisguiseKey(raw string) (*DisguiseKey, error)` — 校验：JSON 可解析、type 合法、该 type 必填字段非空；OAuth 允许 access_token 为空（懒刷新）但 refresh_token 非空
  - `func (k *DisguiseKey) Validate() error`
- Consumes: 无

- [ ] **Step 1: 写失败测试**

```go
package codexdisguise

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDisguiseKeySub2API(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"sub2api","api_key":"sk-sub2api-123"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeSub2API, key.Type)
	assert.Equal(t, "sk-sub2api-123", key.APIKey)
}

func TestParseDisguiseKeyOAuth(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"oauth","access_token":"at","refresh_token":"rt","account_id":"acc_1"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeOAuth, key.Type)
	assert.Equal(t, "acc_1", key.AccountID)
}

func TestParseDisguiseKeyOAuthAllowsEmptyAccessToken(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"oauth","refresh_token":"rt","account_id":"acc_1"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeOAuth, key.Type)
	assert.Empty(t, key.AccessToken)
}

func TestParseDisguiseKeyAgent(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"agent","agent_private_key":"pk","agent_runtime_id":"rid","agent_task_id":"tid"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeAgent, key.Type)
	assert.Equal(t, "rid", key.AgentRuntimeID)
}

func TestParseDisguiseKeyRejectsNonJSON(t *testing.T) {
	_, err := ParseDisguiseKey("sk-plain-text")
	require.Error(t, err)
}

func TestParseDisguiseKeyRejectsUnknownType(t *testing.T) {
	_, err := ParseDisguiseKey(`{"type":"unknown","api_key":"x"}`)
	require.Error(t, err)
}

func TestParseDisguiseKeyRejectsMissingRequired(t *testing.T) {
	_, err := ParseDisguiseKey(`{"type":"sub2api"}`)
	require.Error(t, err)
	_, err = ParseDisguiseKey(`{"type":"oauth","refresh_token":"rt"}`)
	require.NoError(t, err, "oauth 允许缺 account_id（刷新后从 JWT 提取）")
	_, err = ParseDisguiseKey(`{"type":"oauth"}`)
	require.Error(t, err, "oauth 必须至少一个 token")
	_, err = ParseDisguiseKey(`{"type":"agent","agent_private_key":"pk"}`)
	require.Error(t, err)
}

func TestParseDisguiseKeyTrimsValues(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"sub2api","api_key":"  sk-x  "}`)
	require.NoError(t, err)
	assert.Equal(t, "sk-x", key.APIKey)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestParseDisguiseKey -v`
Expected: 编译失败（包/函数不存在）

- [ ] **Step 3: 实现 key.go**

```go
package codexdisguise

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type DisguiseKeyType string

const (
	DisguiseKeyTypeSub2API DisguiseKeyType = "sub2api"
	DisguiseKeyTypeOAuth   DisguiseKeyType = "oauth"
	DisguiseKeyTypeAgent   DisguiseKeyType = "agent"
)

type DisguiseKey struct {
	Type            DisguiseKeyType `json:"type"`
	APIKey          string          `json:"api_key,omitempty"`
	AccessToken     string          `json:"access_token,omitempty"`
	RefreshToken    string          `json:"refresh_token,omitempty"`
	AccountID       string          `json:"account_id,omitempty"`
	AgentPrivateKey string          `json:"agent_private_key,omitempty"`
	AgentRuntimeID  string          `json:"agent_runtime_id,omitempty"`
	AgentTaskID     string          `json:"agent_task_id,omitempty"`
}

func ParseDisguiseKey(raw string) (*DisguiseKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("codex disguise channel: empty key")
	}
	var key DisguiseKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("codex disguise channel: key must be a JSON object")
	}
	key.APIKey = strings.TrimSpace(key.APIKey)
	key.AccessToken = strings.TrimSpace(key.AccessToken)
	key.RefreshToken = strings.TrimSpace(key.RefreshToken)
	key.AccountID = strings.TrimSpace(key.AccountID)
	key.AgentPrivateKey = strings.TrimSpace(key.AgentPrivateKey)
	key.AgentRuntimeID = strings.TrimSpace(key.AgentRuntimeID)
	key.AgentTaskID = strings.TrimSpace(key.AgentTaskID)
	if err := key.Validate(); err != nil {
		return nil, err
	}
	return &key, nil
}

func (k *DisguiseKey) Validate() error {
	if k == nil {
		return errors.New("codex disguise channel: nil key")
	}
	switch k.Type {
	case DisguiseKeyTypeSub2API:
		if k.APIKey == "" {
			return errors.New("codex disguise channel: api_key is required for sub2api type")
		}
	case DisguiseKeyTypeOAuth:
		if k.AccessToken == "" && k.RefreshToken == "" {
			return errors.New("codex disguise channel: access_token or refresh_token is required for oauth type")
		}
	case DisguiseKeyTypeAgent:
		if k.AgentPrivateKey == "" || k.AgentRuntimeID == "" || k.AgentTaskID == "" {
			return errors.New("codex disguise channel: agent_private_key, agent_runtime_id and agent_task_id are required for agent type")
		}
	default:
		return errors.New("codex disguise channel: unknown key type: " + string(k.Type))
	}
	return nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./relay/channel/codexdisguise/ -run TestParseDisguiseKey -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add relay/channel/codexdisguise/key.go relay/channel/codexdisguise/key_test.go
git commit -m "feat(codex-disguise): add multi-format disguise key parsing"
```

---

### Task 3: 身份三元组（identity.go）

**Files:**
- Create: `relay/channel/codexdisguise/identity.go`
- Create: `relay/channel/codexdisguise/identity_test.go`

**Interfaces:**
- Produces:
  - `const codexCLIVersion = "0.146.0"`（编译期兜底）
  - `const codexCLIUserAgentSuffix = " (Ubuntu 22.4.0; x86_64) xterm-256color"`
  - `const codexUpstreamMinVersion = "0.144.0"`
  - `const CodexDefaultOriginator = "codex-tui"`
  - `func NormalizeCodexClientVersion(version string) string`（正则 `^[0-9]+(\.[0-9]+){1,3}(-[0-9A-Za-z.]+)?$`，≤64 字符）
  - `func buildCodexCLIUserAgent(version string) string`
  - `func compareCodexVersions(a, b string) int`（三段数字比较，忽略预发布后缀）
  - `func pairCodexClientIdentity(userAgent string) (originator, pairedUA string, ok bool)`
  - `func codexUserAgentVersion(userAgent string) string`
  - `func setCodexUserAgentVersion(userAgent, version string) string`
  - `type codexOutboundIdentity struct { userAgent, originator, version string }`
  - `func resolveCodexOutboundIdentity(candidateUA, canonicalUA, version string) codexOutboundIdentity`
  - `func enforceCodexIdentityHeaders(h http.Header, canonicalUA, version string, enforce bool)`
  - `func ensureCodexIdentityHeaders(h http.Header, canonicalUA, version string)`
- Consumes: `DisguiseKey`（Task 2）

- [ ] **Step 1: 写失败测试**

```go
package codexdisguise

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCodexClientVersion(t *testing.T) {
	assert.Equal(t, "0.146.0", NormalizeCodexClientVersion(" 0.146.0 "))
	assert.Equal(t, "0.147.0-alpha.4", NormalizeCodexClientVersion("0.147.0-alpha.4"))
	assert.Empty(t, NormalizeCodexClientVersion(""))
	assert.Empty(t, NormalizeCodexClientVersion("v1.2"))
	assert.Empty(t, NormalizeCodexClientVersion("1.2"))
	assert.Empty(t, NormalizeCodexClientVersion("0.146.0; rm -rf /"))
}

func TestCompareCodexVersions(t *testing.T) {
	assert.Equal(t, 0, compareCodexVersions("0.146.0", "0.146.0"))
	assert.Equal(t, -1, compareCodexVersions("0.145.0", "0.146.0"))
	assert.Equal(t, 1, compareCodexVersions("0.147.0", "0.146.0"))
	assert.Equal(t, -1, compareCodexVersions("0.146.0", "0.146.1"))
	assert.Equal(t, 0, compareCodexVersions("0.146.0", "0.146.0-alpha.1"))
}

func TestPairCodexClientIdentity(t *testing.T) {
	// UA 首段是官方 originator → 直接配对
	originator, ua, ok := pairCodexClientIdentity("codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color")
	require.True(t, ok)
	assert.Equal(t, "codex-tui", originator)
	assert.Equal(t, "codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color", ua)

	// 非官方首段 + 官方尾部 (name; version) → 尾部恢复
	originator, ua, ok = pairCodexClientIdentity("cccc/0.142.0 (Ubuntu 22.4.0; x86_64) xterm-256color (codex-tui; 0.142.0)")
	require.True(t, ok)
	assert.Equal(t, "codex-tui", originator)
	assert.Equal(t, "codex-tui/0.142.0 (Ubuntu 22.4.0; x86_64) xterm-256color (codex-tui; 0.142.0)", ua)

	// 完全非官方 → false
	_, _, ok = pairCodexClientIdentity("evil-client/1.0 (Linux; x86_64) xterm")
	assert.False(t, ok)
}

func TestCodexUserAgentVersion(t *testing.T) {
	assert.Equal(t, "0.146.0", codexUserAgentVersion("codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64)"))
	assert.Equal(t, "0.147.0-alpha.4", codexUserAgentVersion("codex-tui/0.147.0-alpha.4 (Ubuntu 22.4.0; x86_64)"))
	assert.Empty(t, codexUserAgentVersion("not-a-codex-ua"))
}

func TestSetCodexUserAgentVersion(t *testing.T) {
	got := setCodexUserAgentVersion("codex-tui/0.142.0 (Ubuntu 22.4.0; x86_64) xterm-256color (codex-tui; 0.142.0)", "0.146.0")
	assert.Equal(t, "codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color (codex-tui; 0.146.0)", got)
	// 非 Codex 形态 UA 返回空串
	assert.Empty(t, setCodexUserAgentVersion("curl/8.0", "0.146.0"))
}

func TestResolveCodexOutboundIdentity(t *testing.T) {
	canonical := buildCodexCLIUserAgent("0.146.0")
	identity := resolveCodexOutboundIdentity("", canonical, "0.146.0")
	assert.Equal(t, canonical, identity.userAgent)
	assert.Equal(t, CodexDefaultOriginator, identity.originator)
	assert.Equal(t, "0.146.0", identity.version)
}

func TestResolveCodexOutboundIdentityRebuildsStaleVersion(t *testing.T) {
	canonical := buildCodexCLIUserAgent("0.146.0")
	// 候选 UA 自带旧版本 → 版本段被重建为生效版本
	identity := resolveCodexOutboundIdentity("codex-tui/0.142.0 (Ubuntu 22.4.0; x86_64)", canonical, "0.146.0")
	assert.Equal(t, "0.146.0", identity.version)
	assert.Contains(t, identity.userAgent, "codex-tui/0.146.0")
}

func TestEnforceCodexIdentityHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("originator", "evil")
	h.Set("user-agent", "evil/1.0")
	h.Set("version", "0.1.0")
	enforceCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0", true)
	assert.Equal(t, "codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color", h.Get("user-agent"))
	assert.Equal(t, "codex-tui", h.Get("originator"))
	assert.Equal(t, "0.146.0", h.Get("version"))
}

func TestEnforceCodexIdentityHeadersSkipsWithoutOriginator(t *testing.T) {
	h := http.Header{}
	h.Set("user-agent", "evil/1.0")
	enforceCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0", true)
	assert.Equal(t, "evil/1.0", h.Get("user-agent"), "无 originator 的请求不被补回身份")
}

func TestEnforceCodexIdentityHeadersDisabledPairsOnly(t *testing.T) {
	h := http.Header{}
	h.Set("originator", "codex-vscode")
	h.Set("user-agent", "codex-vscode/1.0.0 (Ubuntu 22.4.0; x86_64)")
	h.Set("version", "0.1.0")
	enforceCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0", false)
	assert.Equal(t, "codex-vscode", h.Get("originator"), "关闭强制统一保留客户端身份")
	assert.Equal(t, "0.146.0", h.Get("version"), "版本仍被门槛校正")
}

func TestEnsureCodexIdentityHeaders(t *testing.T) {
	h := http.Header{}
	ensureCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0")
	assert.Equal(t, buildCodexCLIUserAgent("0.146.0"), h.Get("user-agent"))
	assert.Equal(t, "codex-tui", h.Get("originator"))
	assert.Equal(t, "0.146.0", h.Get("version"))
	assert.Equal(t, "responses=experimental", h.Get("OpenAI-Beta"))
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestNormalize|TestCompare|TestPair|TestCodexUserAgent|TestSetCodex|TestResolve|TestEnforce|TestEnsure' -v`
Expected: 编译失败（符号未定义）

- [ ] **Step 3: 实现 identity.go**

```go
package codexdisguise

import (
	"net/http"
	"regexp"
	"strings"
)

// codexUpstreamMinVersion 上游 /backend-api/codex 接受的最低 version 头：
// 若请求携带 version 且低于该值，上游直接 404（issue #3901）。
const codexUpstreamMinVersion = "0.144.0"

// codexCLIVersion 编译期兜底版本，跟随官方 Codex CLI 当前发布版。
const codexCLIVersion = "0.146.0"

// codexCLIUserAgentSuffix 对齐真实 Codex TUI 的 OS / 架构 / 终端指纹。
const codexCLIUserAgentSuffix = " (Ubuntu 22.4.0; x86_64) xterm-256color"

// CodexDefaultOriginator 默认 originator（交互式 TUI）。
const CodexDefaultOriginator = "codex-tui"

const codexClientVersionMaxLen = 64

var codexClientVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}(-[0-9A-Za-z.]+)?$`)

var codexOfficialClientOriginators = map[string]bool{
	"codex_cli_rs":          true,
	"codex-tui":             true,
	"codex_vscode":          true,
	"codex_vscode_copilot":  true,
	"codex_app":             true,
	"codex_chatgpt_desktop": true,
	"codex_atlas":           true,
	"codex_exec":            true,
	"codex_sdk_ts":          true,
}

func NormalizeCodexClientVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || len(version) > codexClientVersionMaxLen || !codexClientVersionPattern.MatchString(version) {
		return ""
	}
	return version
}

func buildCodexCLIUserAgent(version string) string {
	if version = NormalizeCodexClientVersion(version); version == "" {
		version = codexCLIVersion
	}
	return CodexDefaultOriginator + "/" + version + codexCLIUserAgentSuffix
}

// compareCodexVersions 比较两段版本号（三段数字），忽略预发布后缀。
func compareCodexVersions(a, b string) int {
	pa, pb := codexVersionParts(a), codexVersionParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func codexVersionParts(version string) [3]int {
	var parts [3]int
	segments := strings.SplitN(strings.TrimSpace(version), ".", 3)
	for i := 0; i < len(segments) && i < 3; i++ {
		n := 0
		for _, ch := range segments[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		parts[i] = n
	}
	return parts
}

func isCodexOfficialOriginator(name string) bool {
	v := strings.ToLower(strings.TrimSpace(name))
	if v == "" || len(v) > 64 {
		return false
	}
	return codexOfficialClientOriginators[v]
}

func codexUATrailerName(ua string) string {
	last := strings.LastIndex(ua, "(")
	if last < 0 {
		return ""
	}
	rest := ua[last+1:]
	closeIdx := strings.Index(rest, ")")
	if closeIdx < 0 {
		return ""
	}
	inner := strings.TrimSpace(rest[:closeIdx])
	if semi := strings.Index(inner, ";"); semi >= 0 {
		inner = strings.TrimSpace(inner[:semi])
	}
	return inner
}

// pairCodexClientIdentity 由出站 UA 推导配套 originator：
// 1. UA 首段是官方 originator → 直接配对；2. UA 尾部 (name; version) 的 name 是官方 → 重写首段。
func pairCodexClientIdentity(userAgent string) (originator string, pairedUA string, ok bool) {
	ua := strings.TrimSpace(userAgent)
	slash := strings.IndexByte(ua, '/')
	if slash <= 0 {
		return "", "", false
	}
	if leading := strings.TrimSpace(ua[:slash]); isCodexOfficialOriginator(leading) {
		return leading, leading + ua[slash:], true
	}
	if trailer := codexUATrailerName(ua); trailer != "" && !strings.ContainsRune(trailer, '/') && isCodexOfficialOriginator(trailer) {
		return trailer, trailer + ua[slash:], true
	}
	return "", "", false
}

// codexUserAgentVersion 提取 UA 的完整版本段（`{client}/{version} (...`）。
func codexUserAgentVersion(userAgent string) string {
	ua := strings.TrimSpace(userAgent)
	slash := strings.IndexByte(ua, '/')
	if slash <= 0 {
		return ""
	}
	rest := ua[slash+1:]
	if space := strings.IndexByte(rest, ' '); space >= 0 {
		rest = rest[:space]
	}
	return strings.TrimSpace(rest)
}

// setCodexUserAgentVersion 重建 UA 中的版本声明（首段 + 尾部官方标识组），
// 其余部分原样保留；UA 不是 `{client}/{version}` 形态时返回空串。
func setCodexUserAgentVersion(userAgent, version string) string {
	ua := strings.TrimSpace(userAgent)
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	slash := strings.IndexByte(ua, '/')
	if slash <= 0 {
		return ""
	}
	client := strings.TrimSpace(ua[:slash])
	if client == "" {
		return ""
	}
	rest := ua[slash+1:]
	tail := ""
	if space := strings.IndexByte(rest, ' '); space >= 0 {
		tail = rest[space:]
	} else if strings.TrimSpace(rest) == "" {
		return ""
	}
	return rewriteCodexUATrailerVersion(client+"/"+version+tail, version)
}

func rewriteCodexUATrailerVersion(ua, version string) string {
	open := strings.LastIndex(ua, "(")
	if open < 0 {
		return ua
	}
	closeIdx := strings.Index(ua[open+1:], ")")
	if closeIdx < 0 {
		return ua
	}
	inner := ua[open+1 : open+1+closeIdx]
	semi := strings.Index(inner, ";")
	if semi < 0 {
		return ua
	}
	name := strings.TrimSpace(inner[:semi])
	if name == "" || !isCodexOfficialOriginator(name) {
		return ua
	}
	return ua[:open+1] + name + "; " + version + ua[open+1+closeIdx:]
}

type codexOutboundIdentity struct {
	userAgent  string
	originator string
	version    string
}

// resolveCodexOutboundIdentity 由候选 UA 推导自洽的出站身份三元组。
// candidateUA 为空时使用 canonicalUA；推导不出官方身份时整体回退规范身份。
// 版本段一律用 version 重建（不允许陈旧版本钉死）。
func resolveCodexOutboundIdentity(candidateUA, canonicalUA, version string) codexOutboundIdentity {
	ua := strings.TrimSpace(candidateUA)
	if ua == "" {
		ua = canonicalUA
	}
	originator, pairedUA, ok := pairCodexClientIdentity(ua)
	if !ok {
		if originator, pairedUA, ok = pairCodexClientIdentity(canonicalUA); !ok {
			originator, pairedUA = CodexDefaultOriginator, buildCodexCLIUserAgent(version)
		}
	}
	if rebuilt := setCodexUserAgentVersion(pairedUA, version); rebuilt != "" {
		pairedUA = rebuilt
	}
	return codexOutboundIdentity{userAgent: pairedUA, originator: originator, version: version}
}

// enforceCodexIdentityHeaders 强制统一出站身份（enforce=true）或退回配套收口（enforce=false）。
// 仅对携带 originator 的请求生效；必须在所有 UA 改写之后调用。
func enforceCodexIdentityHeaders(h http.Header, canonicalUA, version string, enforce bool) {
	if h == nil || h.Get("originator") == "" {
		return
	}
	identity := resolveCodexOutboundIdentity("", canonicalUA, version)
	if !enforce {
		pairCodexIdentityHeaders(h, identity)
		return
	}
	h.Set("user-agent", identity.userAgent)
	h.Set("originator", identity.originator)
	h.Set("version", identity.version)
}

func pairCodexIdentityHeaders(h http.Header, identity codexOutboundIdentity) {
	originator, pairedUA, ok := pairCodexClientIdentity(h.Get("user-agent"))
	if !ok {
		originator, pairedUA = identity.originator, identity.userAgent
		h.Set("version", identity.version)
	}
	h.Set("user-agent", pairedUA)
	h.Set("originator", originator)
	if v := strings.TrimSpace(h.Get("version")); v != "" && compareCodexVersions(v, codexUpstreamMinVersion) < 0 {
		h.Set("version", identity.version)
	}
}

// ensureCodexIdentityHeaders 补齐缺失的身份头。
func ensureCodexIdentityHeaders(h http.Header, canonicalUA, version string) {
	if h == nil {
		return
	}
	identity := resolveCodexOutboundIdentity("", canonicalUA, version)
	if strings.TrimSpace(h.Get("user-agent")) == "" {
		h.Set("user-agent", identity.userAgent)
	}
	if strings.TrimSpace(h.Get("originator")) == "" {
		h.Set("originator", identity.originator)
	}
	if strings.TrimSpace(h.Get("version")) == "" {
		h.Set("version", identity.version)
	}
	h.Set("OpenAI-Beta", "responses=experimental")
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestNormalize|TestCompare|TestPair|TestCodexUserAgent|TestSetCodex|TestResolve|TestEnforce|TestEnsure' -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add relay/channel/codexdisguise/identity.go relay/channel/codexdisguise/identity_test.go
git commit -m "feat(codex-disguise): add codex outbound identity triad"
```

---

### Task 4: 指纹收敛（fingerprint.go）

**Files:**
- Create: `relay/channel/codexdisguise/fingerprint.go`
- Create: `relay/channel/codexdisguise/fingerprint_test.go`

**Interfaces:**
- Produces:
  - `type FingerprintMode string`；常量 `FingerprintModeOff/Device/Session/Full`
  - `func ParseFingerprintMode(raw string) FingerprintMode`（未知 → off）
  - `func ValidateFingerprintSeed(seed string) bool`（规范 UUIDv4）
  - `func deriveStableUUIDv4(seed string) string`
  - `type codexFingerprintIDs struct { Mode FingerprintMode; InstallationID, SessionID, ThreadID, TurnID, WindowID string; TurnStartedAtUnixMs int64 }`
  - `func resolveCodexFingerprintIDs(mode FingerprintMode, seed, clientSessionID string) *codexFingerprintIDs`（off/无 seed → nil）
  - `func applyCodexFingerprintHeaders(h http.Header, ids *codexFingerprintIDs)`
  - `func applyCodexFingerprintClientMetadata(reqBody map[string]any, ids *codexFingerprintIDs) bool`（含内嵌 x-codex-turn-metadata 同步改写）
- Consumes: 无（纯函数）

- [ ] **Step 1: 写失败测试**

```go
package codexdisguise

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFingerprintMode(t *testing.T) {
	assert.Equal(t, FingerprintModeOff, ParseFingerprintMode(""))
	assert.Equal(t, FingerprintModeOff, ParseFingerprintMode("bogus"))
	assert.Equal(t, FingerprintModeDevice, ParseFingerprintMode("device"))
	assert.Equal(t, FingerprintModeSession, ParseFingerprintMode("session"))
	assert.Equal(t, FingerprintModeFull, ParseFingerprintMode("full"))
}

func TestValidateFingerprintSeed(t *testing.T) {
	assert.True(t, ValidateFingerprintSeed("3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f"))
	assert.False(t, ValidateFingerprintSeed(""))
	assert.False(t, ValidateFingerprintSeed("not-a-uuid"))
	assert.False(t, ValidateFingerprintSeed("3f2e1a9c-8b4d-1f6a-9e1c-2d3b4c5d6e7f"))
}

func TestDeriveStableUUIDv4Deterministic(t *testing.T) {
	a := deriveStableUUIDv4("seed-1")
	b := deriveStableUUIDv4("seed-1")
	c := deriveStableUUIDv4("seed-2")
	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	require.Len(t, a, 36)
	assert.Equal(t, "4", string(a[14]), "UUIDv4 version 位")
}

func TestResolveFingerprintIDsOffReturnsNil(t *testing.T) {
	assert.Nil(t, resolveCodexFingerprintIDs(FingerprintModeOff, "seed", "client-session"))
	assert.Nil(t, resolveCodexFingerprintIDs(FingerprintModeSession, "", "client-session"))
}

func TestResolveFingerprintIDsSession(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeSession, "seed", "client-session-1")
	require.NotNil(t, ids)
	assert.Equal(t, FingerprintModeSession, ids.Mode)
	assert.NotEmpty(t, ids.InstallationID)
	assert.NotEmpty(t, ids.SessionID)
	assert.NotEmpty(t, ids.ThreadID)
	assert.NotEmpty(t, ids.TurnID)
	assert.Equal(t, ids.ThreadID+":0", ids.WindowID)
	assert.NotEmpty(t, ids.TurnStartedAtUnixMs)
	// 同 seed 同 clientSessionID → 同 thread
	ids2 := resolveCodexFingerprintIDs(FingerprintModeSession, "seed", "client-session-1")
	assert.Equal(t, ids.ThreadID, ids2.ThreadID)
	// 不同 clientSessionID → 不同 thread
	ids3 := resolveCodexFingerprintIDs(FingerprintModeSession, "seed", "client-session-2")
	assert.NotEqual(t, ids.ThreadID, ids3.ThreadID)
}

func TestResolveFingerprintIDsFullThreadEqualsSession(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeFull, "seed", "anything")
	require.NotNil(t, ids)
	assert.Equal(t, ids.SessionID, ids.ThreadID)
}

func TestResolveFingerprintIDsDeviceOnlyInstallation(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeDevice, "seed", "client-session")
	require.NotNil(t, ids)
	assert.NotEmpty(t, ids.InstallationID)
	assert.Empty(t, ids.SessionID)
	assert.Empty(t, ids.ThreadID)
}

func TestApplyFingerprintHeadersSession(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeSession, "seed", "cs")
	require.NotNil(t, ids)
	h := http.Header{}
	h.Set("session-id", "original")
	h.Set("session_id", "original")
	h.Set("x-codex-turn-metadata", `{"installation_id":"old","session_id":"old","thread_id":"old","turn_id":"old","window_id":"old","turn_started_at_unix_ms":0,"sandbox":"keep"}`)
	applyCodexFingerprintHeaders(h, ids)
	assert.Equal(t, ids.InstallationID, h.Get("x-codex-installation-id"))
	assert.Equal(t, ids.SessionID, h.Get("session-id"))
	assert.Equal(t, ids.SessionID, h.Get("session_id"))
	assert.Equal(t, ids.ThreadID, h.Get("thread-id"))
	assert.Equal(t, ids.ThreadID, h.Get("x-client-request-id"))
	assert.Equal(t, ids.WindowID, h.Get("x-codex-window-id"))
	metadata := h.Get("x-codex-turn-metadata")
	assert.Contains(t, metadata, ids.TurnID)
	assert.Contains(t, metadata, `"sandbox":"keep"`)
}

func TestApplyFingerprintClientMetadata(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeSession, "seed", "cs")
	require.NotNil(t, ids)
	body := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-installation-id": "old",
			"session_id":              "old",
			"thread_id":               "old",
			"turn_id":                 "old",
			"x-codex-window-id":       "old",
			"x-codex-turn-metadata":   `{"installation_id":"old","turn_id":"old"}`,
		},
	}
	modified := applyCodexFingerprintClientMetadata(body, ids)
	assert.True(t, modified)
	cm, ok := body["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ids.InstallationID, cm["x-codex-installation-id"])
	assert.Equal(t, ids.SessionID, cm["session_id"])
	assert.Equal(t, ids.ThreadID, cm["thread_id"])
	assert.Equal(t, ids.TurnID, cm["turn_id"])
	assert.Contains(t, cm["x-codex-turn-metadata"].(string), ids.TurnID)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestParseFingerprint|TestValidateFingerprint|TestDeriveStable|TestResolveFingerprint|TestApplyFingerprint' -v`
Expected: 编译失败（符号未定义）

- [ ] **Step 3: 实现 fingerprint.go**

```go
package codexdisguise

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

type FingerprintMode string

const (
	FingerprintModeOff    FingerprintMode = "off"
	FingerprintModeDevice FingerprintMode = "device"
	FingerprintModeSession FingerprintMode = "session"
	FingerprintModeFull   FingerprintMode = "full"
)

func ParseFingerprintMode(raw string) FingerprintMode {
	switch FingerprintMode(strings.TrimSpace(raw)) {
	case FingerprintModeOff, FingerprintModeDevice, FingerprintModeSession, FingerprintModeFull:
		return FingerprintMode(strings.TrimSpace(raw))
	default:
		return FingerprintModeOff
	}
}

func ValidateFingerprintSeed(seed string) bool {
	trimmed := strings.TrimSpace(seed)
	parsed, err := uuid.Parse(trimmed)
	return err == nil && parsed != uuid.Nil && trimmed == parsed.String()
}

// deriveStableUUIDv4 从种子确定性派生 UUIDv4 格式字符串，同种子永远同值。
func deriveStableUUIDv4(seed string) string {
	h := sha256.Sum256([]byte(seed))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16])
}

type codexFingerprintIDs struct {
	Mode               FingerprintMode
	InstallationID     string
	SessionID          string
	ThreadID           string
	TurnID             string
	WindowID           string
	TurnStartedAtUnixMs int64
}

func resolveCodexFingerprintIDs(mode FingerprintMode, seed, clientSessionID string) *codexFingerprintIDs {
	if mode == FingerprintModeOff || !ValidateFingerprintSeed(seed) {
		return nil
	}
	ids := &codexFingerprintIDs{
		Mode:                mode,
		InstallationID:      deriveStableUUIDv4("new-api:codex-install-id:v2:" + seed),
		TurnStartedAtUnixMs: time.Now().UnixMilli(),
	}
	if mode == FingerprintModeDevice {
		return ids
	}
	ids.SessionID = deriveStableUUIDv4("new-api:codex-session-id:v2:" + seed)
	switch mode {
	case FingerprintModeSession:
		if clientSessionID != "" {
			ids.ThreadID = deriveStableUUIDv4("new-api:codex-thread-id:v2:" + seed + ":" + clientSessionID)
		}
		if ids.ThreadID == "" {
			ids.ThreadID = ids.SessionID
		}
	case FingerprintModeFull:
		ids.ThreadID = ids.SessionID
	}
	ids.TurnID = uuid.Must(uuid.NewV7()).String()
	ids.WindowID = ids.ThreadID + ":0"
	return ids
}

// applyCodexFingerprintHeaders 按收敛 ID 改写出站头。
func applyCodexFingerprintHeaders(h http.Header, ids *codexFingerprintIDs) {
	if h == nil || ids == nil {
		return
	}
	h.Set("x-codex-installation-id", ids.InstallationID)
	if ids.Mode == FingerprintModeDevice {
		rewriteCodexTurnMetadataFields(h, map[string]any{"installation_id": ids.InstallationID})
		return
	}
	h.Set("x-codex-window-id", ids.WindowID)
	h.Set("x-client-request-id", ids.ThreadID)
	h.Set("session-id", ids.SessionID)
	h.Set("session_id", ids.SessionID)
	h.Set("thread-id", ids.ThreadID)
	rewriteCodexTurnMetadataFields(h, map[string]any{
		"installation_id":         ids.InstallationID,
		"session_id":              ids.SessionID,
		"thread_id":               ids.ThreadID,
		"turn_id":                 ids.TurnID,
		"window_id":               ids.WindowID,
		"turn_started_at_unix_ms": ids.TurnStartedAtUnixMs,
	})
}

func rewriteCodexTurnMetadataFields(h http.Header, fields map[string]any) {
	raw := strings.TrimSpace(h.Get("x-codex-turn-metadata"))
	if raw == "" {
		return
	}
	var metadata map[string]any
	if err := common.Unmarshal([]byte(raw), &metadata); err != nil || metadata == nil {
		metadata = make(map[string]any, len(fields))
	}
	for k, v := range fields {
		metadata[k] = v
	}
	rebuilt, err := common.Marshal(metadata)
	if err != nil {
		return
	}
	h.Set("x-codex-turn-metadata", string(rebuilt))
}

// applyCodexFingerprintClientMetadata 按收敛 ID 改写请求体 client_metadata 与内嵌 turn-metadata。
func applyCodexFingerprintClientMetadata(reqBody map[string]any, ids *codexFingerprintIDs) bool {
	if reqBody == nil || ids == nil {
		return false
	}
	existing, _ := reqBody["client_metadata"].(map[string]any)
	if existing == nil {
		existing = make(map[string]any)
	}
	modified := false
	if applyCodexFingerprintToClientMetadataMap(existing, ids) {
		reqBody["client_metadata"] = existing
		modified = true
	}
	return modified
}

func applyCodexFingerprintToClientMetadataMap(existing map[string]any, ids *codexFingerprintIDs) bool {
	changed := false
	set := func(k string, v any) {
		if existing[k] != v {
			existing[k] = v
			changed = true
		}
	}
	set("x-codex-installation-id", ids.InstallationID)
	if ids.Mode != FingerprintModeDevice {
		set("session_id", ids.SessionID)
		set("thread_id", ids.ThreadID)
		set("turn_id", ids.TurnID)
		set("x-codex-window-id", ids.WindowID)
		if embedded, ok := existing["x-codex-turn-metadata"].(string); ok && embedded != "" {
			var embeddedMap map[string]any
			if err := common.Unmarshal([]byte(embedded), &embeddedMap); err == nil {
				rewriteCodexTurnMetadataFieldsMap(embeddedMap, ids)
				if rebuilt, err := common.Marshal(embeddedMap); err == nil {
					existing["x-codex-turn-metadata"] = string(rebuilt)
					changed = true
				}
			}
		}
	}
	return changed
}

func rewriteCodexTurnMetadataFieldsMap(m map[string]any, ids *codexFingerprintIDs) {
	fields := map[string]any{
		"installation_id":         ids.InstallationID,
		"session_id":              ids.SessionID,
		"thread_id":               ids.ThreadID,
		"turn_id":                 ids.TurnID,
		"window_id":               ids.WindowID,
		"turn_started_at_unix_ms": ids.TurnStartedAtUnixMs,
	}
	for k, v := range fields {
		m[k] = v
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestParseFingerprint|TestValidateFingerprint|TestDeriveStable|TestResolveFingerprint|TestApplyFingerprint' -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add relay/channel/codexdisguise/fingerprint.go relay/channel/codexdisguise/fingerprint_test.go
git commit -m "feat(codex-disguise): add codex fingerprint convergence"
```

---

### Task 5: turn-state 溯源与剥离（turn_state.go）

**Files:**
- Create: `relay/channel/codexdisguise/turn_state.go`
- Create: `relay/channel/codexdisguise/turn_state_test.go`

**Interfaces:**
- Produces:
  - `type turnStateRegistry struct { mu sync.Mutex; entries map[string]turnStateEntry; ttl time.Duration }`
  - `type turnStateEntry struct { threadID string; mintedAt time.Time }`
  - `func newTurnStateRegistry(ttl time.Duration) *turnStateRegistry`
  - `func (r *turnStateRegistry) note(threadID, blob string)` — 记录"该 blob 由哪个 thread 铸造"
  - `func (r *turnStateRegistry) guard(threadID, blob string) string` — 返回应出站的 blob；blob 为空 → 原样；未知 → 记录并放行；已知但铸造 thread ≠ 当前 → 剥离返回 ""；铸造 thread == 当前 → 放行
  - `func (r *turnStateRegistry) sweep()` — 惰性清理过期条目
- Consumes: 无

- [ ] **Step 1: 写失败测试**

```go
package codexdisguise

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurnStateRegistryUnknownBlobPassesThrough(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-1"
	assert.Equal(t, blob, r.guard("thread-1", blob), "未知 blob 放行并记录")
	entry, ok := r.entries[blob]
	require.True(t, ok)
	assert.Equal(t, "thread-1", entry.threadID)
}

func TestTurnStateRegistrySameThreadPasses(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-2"
	r.guard("thread-1", blob)
	assert.Equal(t, blob, r.guard("thread-1", blob))
}

func TestTurnStateRegistryCrossThreadStrips(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-3"
	r.guard("thread-1", blob)
	assert.Empty(t, r.guard("thread-2", blob), "异 thread 回带剥离")
}

func TestTurnStateRegistryExpiredEntryForgotten(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-4"
	r.guard("thread-1", blob)
	r.entries[blob] = turnStateEntry{threadID: "thread-1", mintedAt: time.Now().Add(-2 * time.Hour)}
	r.sweep()
	_, ok := r.entries[blob]
	assert.False(t, ok, "过期条目被清理")
	assert.Equal(t, blob, r.guard("thread-2", blob), "清理后可重新放行")
}

func TestTurnStateRegistryEmptyBlobIgnored(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	assert.Empty(t, r.guard("thread-1", ""))
	assert.Len(t, r.entries, 0)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run TestTurnStateRegistry -v`
Expected: 编译失败（符号未定义）

- [ ] **Step 3: 实现 turn_state.go**

```go
package codexdisguise

import (
	"strings"
	"sync"
	"time"
)

type turnStateEntry struct {
	threadID string
	mintedAt time.Time
}

// turnStateRegistry 记录 x-codex-turn-state 的铸造归属（thread → 铸造方）。
// 多下游共享统一会话时，客户端回带由其他 thread 铸造的 turn-state 是代理链
// 独有的矛盾信号（真实 Codex 不会发生），必须在出站前剥离。
type turnStateRegistry struct {
	mu      sync.Mutex
	entries map[string]turnStateEntry
	ttl     time.Duration
}

func newTurnStateRegistry(ttl time.Duration) *turnStateRegistry {
	return &turnStateRegistry{
		entries: make(map[string]turnStateEntry),
		ttl:     ttl,
	}
}

// note 记录 blob 由指定 thread 铸造（响应侧捕获后调用）。
func (r *turnStateRegistry) note(threadID, blob string) {
	if r == nil || strings.TrimSpace(blob) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	r.entries[blob] = turnStateEntry{threadID: threadID, mintedAt: time.Now()}
}

// guard 返回应出站的 blob：未知 → 记录并放行；同 thread → 放行；异 thread → 剥离（""）。
func (r *turnStateRegistry) guard(threadID, blob string) string {
	blob = strings.TrimSpace(blob)
	if blob == "" {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	entry, ok := r.entries[blob]
	if !ok {
		r.entries[blob] = turnStateEntry{threadID: threadID, mintedAt: time.Now()}
		return blob
	}
	if entry.threadID == threadID {
		return blob
	}
	return ""
}

func (r *turnStateRegistry) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
}

func (r *turnStateRegistry) sweepLocked() {
	if r == nil || r.ttl <= 0 || len(r.entries) == 0 {
		return
	}
	cutoff := time.Now().Add(-r.ttl)
	for blob, entry := range r.entries {
		if entry.mintedAt.Before(cutoff) {
			delete(r.entries, blob)
		}
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./relay/channel/codexdisguise/ -run TestTurnStateRegistry -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add relay/channel/codexdisguise/turn_state.go relay/channel/codexdisguise/turn_state_test.go
git commit -m "feat(codex-disguise): add turn-state provenance registry"
```

---

### Task 6: Agent Identity（agent_identity.go）

**Files:**
- Create: `relay/channel/codexdisguise/agent_identity.go`
- Create: `relay/channel/codexdisguise/agent_identity_test.go`

**Interfaces:**
- Produces:
  - `type agentIdentityKey struct { runtimeID string; privateKey ed25519.PrivateKey; taskID string }`
  - `func parseAgentIdentityPrivateKey(encoded string) (ed25519.PrivateKey, error)`（base64 → PKCS#8 → Ed25519）
  - `func buildAgentAssertion(key agentIdentityKey, now time.Time) (string, error)` — 返回 `"AgentAssertion " + base64url(JSON)`，签名 `ed25519(runtimeID:taskID:timestamp)`
  - `func signAgentTaskRegistration(key agentIdentityKey, timestamp time.Time) (formatted, signature string, err error)` — 签名 `ed25519(runtimeID:RFC3339)`
  - `func decryptAgentTaskID(key agentIdentityKey, encoded string) (string, error)` — 派生 X25519 私钥 → box.OpenAnonymous
- Consumes: `DisguiseKey`（Task 2）

- [ ] **Step 1: 写失败测试**

固定向量：用 `crypto/rand` 在测试内生成 key 并签名验证（不依赖外部 fixture 的私钥材料，避免硬编码密钥）：

```go
package codexdisguise

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

func newTestAgentKey(t *testing.T) agentIdentityKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return agentIdentityKey{runtimeID: "runtime-1", privateKey: priv, taskID: "task-1"}
}

func TestBuildAgentAssertionFormat(t *testing.T) {
	key := newTestAgentKey(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	assertion, err := buildAgentAssertion(key, now)
	require.NoError(t, err)
	require.True(t, len(assertion) > len("AgentAssertion "))
	require.True(t, hasPrefixFold(assertion, "AgentAssertion "))

	encoded := assertion[len("AgentAssertion "):]
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	var envelope map[string]string
	require.NoError(t, json.Unmarshal(raw, &envelope))
	assert.Equal(t, "runtime-1", envelope["agent_runtime_id"])
	assert.Equal(t, "task-1", envelope["task_id"])
	assert.Equal(t, "2026-08-28T12:00:00Z", envelope["timestamp"])

	sig, err := base64.StdEncoding.DecodeString(envelope["signature"])
	require.NoError(t, err)
	payload := []byte("runtime-1:task-1:2026-08-28T12:00:00Z")
	assert.True(t, ed25519.Verify(key.privateKey.Public().(ed25519.PublicKey), payload, sig))
}

func TestBuildAgentAssertionRequiresTask(t *testing.T) {
	key := newTestAgentKey(t)
	key.taskID = ""
	_, err := buildAgentAssertion(key, time.Now())
	require.Error(t, err)
}

func TestSignAgentTaskRegistration(t *testing.T) {
	key := newTestAgentKey(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	formatted, signature, err := signAgentTaskRegistration(key, now)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-28T12:00:00Z", formatted)
	sig, err := base64.StdEncoding.DecodeString(signature)
	require.NoError(t, err)
	payload := []byte("runtime-1:2026-08-28T12:00:00Z")
	assert.True(t, ed25519.Verify(key.privateKey.Public().(ed25519.PublicKey), payload, sig))
}

func TestDecryptAgentTaskIDRoundTrip(t *testing.T) {
	key := newTestAgentKey(t)
	// 用与实现相同的派生路径构造加密方公钥
	seed := key.privateKey.Seed()
	digest := sha512.Sum512(seed)
	var curvePrivate [32]byte
	copy(curvePrivate[:], digest[:32])
	curvePrivate[0] &= 248
	curvePrivate[31] &= 127
	curvePrivate[31] |= 64
	pubBytes, err := curve25519.X25519(curvePrivate[:], curve25519.Basepoint)
	require.NoError(t, err)
	var pub [32]byte
	copy(pub[:], pubBytes)
	plaintext := []byte("task-42")
	sealed, err := box.SealAnonymous(nil, plaintext, &pub, rand.Reader)
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(sealed)

	decrypted, err := decryptAgentTaskID(key, encoded)
	require.NoError(t, err)
	assert.Equal(t, "task-42", decrypted)
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && stringsEqualFold(s[:len(prefix)], prefix)
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestBuildAgent|TestSignAgent|TestDecryptAgent' -v`
Expected: 编译失败（符号未定义，且 `golang.org/x/crypto` 需在 go.mod 中存在——若无则先 `go get golang.org/x/crypto@latest`）

- [ ] **Step 3: 实现 agent_identity.go**

```go
package codexdisguise

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

type agentIdentityKey struct {
	runtimeID  string
	privateKey ed25519.PrivateKey
	taskID     string
}

// parseAgentIdentityPrivateKey 解析 base64 编码的 PKCS#8 Ed25519 私钥。
func parseAgentIdentityPrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw := strings.TrimSpace(encoded)
	if raw == "" {
		return nil, errors.New("agent identity private key is missing")
	}
	der, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("agent identity private key is not valid base64")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, errors.New("agent identity private key is not valid PKCS#8")
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("agent identity private key is not Ed25519")
	}
	return privateKey, nil
}

func agentIdentityKeyFromDisguiseKey(k *DisguiseKey) (agentIdentityKey, error) {
	if k == nil || k.Type != DisguiseKeyTypeAgent {
		return agentIdentityKey{}, errors.New("agent identity key requires agent type key")
	}
	privateKey, err := parseAgentIdentityPrivateKey(k.AgentPrivateKey)
	if err != nil {
		return agentIdentityKey{}, err
	}
	if k.AgentRuntimeID == "" {
		return agentIdentityKey{}, errors.New("agent identity runtime id is missing")
	}
	return agentIdentityKey{
		runtimeID:  k.AgentRuntimeID,
		privateKey: privateKey,
		taskID:     k.AgentTaskID,
	}, nil
}

// buildAgentAssertion 构造 AgentAssertion 认证头：ed25519 签名 runtimeID:taskID:timestamp。
func buildAgentAssertion(key agentIdentityKey, now time.Time) (string, error) {
	if key.runtimeID == "" || key.taskID == "" {
		return "", errors.New("agent identity runtime or task id is missing")
	}
	timestamp := now.UTC().Format(time.RFC3339)
	payload := []byte(key.runtimeID + ":" + key.taskID + ":" + timestamp)
	signature, err := key.privateKey.Sign(nil, payload, crypto.Hash(0))
	if err != nil {
		return "", errors.New("failed to sign agent assertion")
	}
	envelope := map[string]string{
		"agent_runtime_id": key.runtimeID,
		"task_id":          key.taskID,
		"timestamp":        timestamp,
		"signature":        base64.StdEncoding.EncodeToString(signature),
	}
	encoded, err := common.Marshal(envelope)
	if err != nil {
		return "", errors.New("failed to serialize agent assertion")
	}
	return "AgentAssertion " + base64.RawURLEncoding.EncodeToString(encoded), nil
}

// signAgentTaskRegistration 构造 task 注册签名：ed25519 签名 runtimeID:timestamp。
func signAgentTaskRegistration(key agentIdentityKey, timestamp time.Time) (string, string, error) {
	if key.runtimeID == "" {
		return "", "", errors.New("agent identity runtime id is missing")
	}
	formatted := timestamp.UTC().Format(time.RFC3339)
	signature, err := key.privateKey.Sign(nil, []byte(key.runtimeID+":"+formatted), crypto.Hash(0))
	if err != nil {
		return "", "", errors.New("failed to sign agent task registration")
	}
	return formatted, base64.StdEncoding.EncodeToString(signature), nil
}

// decryptAgentTaskID 用派生 X25519 私钥解密 encrypted_task_id。
func decryptAgentTaskID(key agentIdentityKey, encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", errors.New("encrypted agent task id is not valid base64")
	}
	seed := key.privateKey.Seed()
	digest := sha512.Sum512(seed)
	var curvePrivate [32]byte
	copy(curvePrivate[:], digest[:32])
	curvePrivate[0] &= 248
	curvePrivate[31] &= 127
	curvePrivate[31] |= 64
	curvePublicBytes, err := curve25519.X25519(curvePrivate[:], curve25519.Basepoint)
	if err != nil {
		return "", errors.New("failed to derive agent identity decryption key")
	}
	var curvePublic [32]byte
	copy(curvePublic[:], curvePublicBytes)
	plaintext, ok := box.OpenAnonymous(nil, ciphertext, &curvePublic, &curvePrivate)
	if !ok {
		return "", errors.New("failed to decrypt encrypted agent task id")
	}
	taskID := strings.TrimSpace(string(plaintext))
	if taskID == "" {
		return "", errors.New("decrypted agent task id is empty")
	}
	return taskID, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestBuildAgent|TestSignAgent|TestDecryptAgent' -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add relay/channel/codexdisguise/agent_identity.go relay/channel/codexdisguise/agent_identity_test.go
git commit -m "feat(codex-disguise): add agent identity assertion and task registration crypto"
```

---

### Task 7: 渠道配置字段扩展（relaykit/dto）+ 适配器主体（adaptor.go）

**Files:**
- Modify: `relaykit/dto/channel_settings.go:68-88`（ChannelOtherSettings 加伪装配置字段）
- Create: `relay/channel/codexdisguise/adaptor.go`
- Create: `relay/channel/codexdisguise/adaptor_test.go`

**Interfaces:**
- Consumes: `ParseDisguiseKey`/`DisguiseKey`（T2）、`resolveCodexOutboundIdentity` 等（T3）、`resolveCodexFingerprintIDs`/`applyCodexFingerprintHeaders`/`applyCodexFingerprintClientMetadata`（T4）、`turnStateRegistry`（T5）、`agentIdentityKeyFromDisguiseKey`/`buildAgentAssertion`（T6）
- Consumes（主模块已有）: `channel.SetupApiRequestHeader`、`channel.DoApiRequest`、`openai.OaiResponsesHandler`/`OaiResponsesStreamHandler`/`OaiResponsesCompactionHandler`、`relaycommon.GetFullRequestURL`、`relayconstant.RelayMode*`
- Produces: `type Adaptor struct{}` 实现 `channel.Adaptor` 全接口；`ChannelOtherSettings` 新增字段（见 Step 1）

- [ ] **Step 0: 扩展 ChannelOtherSettings（relaykit 模块，必须验证独立构建）**

`relaykit/dto/channel_settings.go` 的 `ChannelOtherSettings` struct 末尾追加：

```go
	// Codex disguise channel settings
	DisguiseEnabled   *bool  `json:"disguise_enabled,omitempty"`    // nil/true = 伪装开启；false = 退化为纯转发
	FingerprintMode   string `json:"fingerprint_mode,omitempty"`    // off/device/session/full；空 = session
	FingerprintSeed   string `json:"fingerprint_seed,omitempty"`    // 渠道级恒定 UUIDv4；空 = 按渠道 ID 派生
	CodexClientVersion string `json:"codex_client_version,omitempty"` // 手配版本；空 = 全局/自动同步
	EnforceIdentity   *bool  `json:"enforce_identity,omitempty"`    // nil/true = 强制统一；false = 仅配套收口
	AgentAutoRegister *bool  `json:"agent_auto_register,omitempty"` // nil/true = task 失效自动重注册
```

验证：Run: `cd relaykit && GOWORK=off go build ./...`（AGENTS.md 强制要求）

**设计要点：**
- `GetRequestURL`：按 RelayMode 拼 `{base}/backend-api/codex/responses`（/compact、/alpha/search）
- `SetupRequestHeader`：`SetupApiRequestHeader` 基础 + 按 Key 类型设置认证头（sub2api → Bearer api_key；oauth → Bearer access_token + chatgpt-account-id；agent → AgentAssertion）→ 身份头补齐/收口 → 指纹头改写（从 gin context 读暂存 IDs）→ turn-state 守卫 → Content-Type/Accept 强制
- `ConvertOpenAIResponsesRequest`：系统提示逻辑（复用现有 codex 渠道语义）+ instructions 缺省 `""` + store=false + 参数剥离；指纹 body 改写（暂存 IDs 到 gin context，供 SetupRequestHeader 复用）
- `DoResponse`：委托 openai 各 handler
- turn-state 捕获：`DoResponse` 中从 `resp.Header` 读取 `x-codex-turn-state`，`registry.note(threadID, blob)` 后由下游处理（SSE handler 透传响应头，无需额外代码——handler 会转发上游响应头；若需显式回传需在 DoResponse 内 `c.Header(...)`）

- [ ] **Step 1: 写失败测试**

```go
package codexdisguise

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRelayInfo(relayMode int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodexDisguise,
			ChannelBaseUrl: "https://sub2api.example.com",
			ApiKey:         `{"type":"sub2api","api_key":"sk-1"}`,
		},
		RelayMode: relayMode,
	}
}

func TestGetRequestURLResponses(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeResponses))
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example.com/backend-api/codex/responses", url)
}

func TestGetRequestURLCompact(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeResponsesCompact))
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example.com/backend-api/codex/responses/compact", url)
}

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeAlphaSearch))
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example.com/backend-api/codex/alpha/search", url)
}

func TestGetRequestURLRejectsUnsupportedMode(t *testing.T) {
	adaptor := &Adaptor{}
	_, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeChatCompletions))
	require.Error(t, err)
}

func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := testRelayInfo(relayconstant.RelayModeResponses)
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            []byte(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: []byte(`1.5`),
		PresencePenalty:  []byte(`1.5`),
	})
	require.NoError(t, err)
	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
	assert.Equal(t, []byte(`false`), []byte(request.Store))
}

func TestConvertOpenAIResponsesRequestDefaultsInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := testRelayInfo(relayconstant.RelayModeResponses)
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model: "gpt-5-codex",
		Input: []byte(`"hello"`),
	})
	require.NoError(t, err)
	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Equal(t, []byte(`""`), []byte(request.Instructions))
}

func TestSetupRequestHeaderSub2APIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.IsStream = true
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk-1", headers.Get("Authorization"))
	assert.Equal(t, "codex-tui", headers.Get("originator"))
	assert.Equal(t, "0.146.0", headers.Get("version"))
	assert.Contains(t, headers.Get("user-agent"), "codex-tui/0.146.0")
	assert.Equal(t, "text/event-stream", headers.Get("Accept"))
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
}

func TestSetupRequestHeaderOAuthKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelMeta.ApiKey = `{"type":"oauth","access_token":"at","refresh_token":"rt","account_id":"acc_1"}`
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	assert.Equal(t, "Bearer at", headers.Get("Authorization"))
	assert.Equal(t, "acc_1", headers.Get("chatgpt-account-id"))
}

func TestSetupRequestHeaderAgentKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 生成真实 Ed25519 key 以便构造有效 DisguiseKey
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyJSON := `{"type":"agent","agent_private_key":"` + base64.StdEncoding.EncodeToString(der) +
		`","agent_runtime_id":"rid","agent_task_id":"tid"}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelMeta.ApiKey = keyJSON
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err = adaptor.SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(headers.Get("Authorization"), "AgentAssertion "))
}

func TestSetupRequestHeaderRejectsPlainKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelMeta.ApiKey = "sk-plain"
	adaptor := &Adaptor{}
	err := adaptor.SetupRequestHeader(c, &headers, info)
	require.Error(t, err)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestGetRequestURL|TestConvertOpenAIResponses|TestSetupRequestHeader' -v`
Expected: 编译失败（Adaptor 未定义）

- [ ] **Step 3: 实现 adaptor.go**

```go
package codexdisguise

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

const (
	codexFingerprintIDsContextKey = "codex_disguise_fingerprint_ids"
	codexThreadIDContextKey       = "codex_disguise_thread_id"
	codexTurnStateBlobContextKey  = "codex_disguise_turn_state_blob"
)

var turnStates = newTurnStateRegistry(1 * 60 * 60 * 1000_000_000)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("codex disguise channel: endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("codex disguise channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("codex disguise channel: endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("codex disguise channel: /v1/chat/completions endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("codex disguise channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	var path string
	switch info.RelayMode {
	case relayconstant.RelayModeResponses:
		path = "/backend-api/codex/responses"
	case relayconstant.RelayModeResponsesCompact:
		path = "/backend-api/codex/responses/compact"
	case relayconstant.RelayModeAlphaSearch:
		path = "/backend-api/codex/alpha/search"
	default:
		return "", errors.New("codex disguise channel: only /v1/responses, /v1/responses/compact and /v1/alpha/search are supported")
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info != nil && info.ChannelSetting.SystemPrompt != "" {
		systemPrompt := info.ChannelSetting.SystemPrompt
		if len(request.Instructions) == 0 {
			if b, err := common.Marshal(systemPrompt); err == nil {
				request.Instructions = b
			} else {
				return nil, err
			}
		} else if info.ChannelSetting.SystemPromptOverride {
			var existing string
			if err := common.Unmarshal(request.Instructions, &existing); err == nil {
				existing = strings.TrimSpace(existing)
				if existing == "" {
					if b, err := common.Marshal(systemPrompt); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				} else {
					if b, err := common.Marshal(systemPrompt + "\n" + existing); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				}
			} else {
				if b, err := common.Marshal(systemPrompt); err == nil {
					request.Instructions = b
				} else {
					return nil, err
				}
			}
		}
	}
	if len(request.Instructions) == 0 {
		request.Instructions = []byte(`""`)
	}

	if info != nil && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		request.Store = []byte("false")
		request.MaxOutputTokens = nil
		request.Temperature = nil
		request.FrequencyPenalty = nil
		request.PresencePenalty = nil
	}

	a.stageCodexFingerprintIDs(c, info)
	return request, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key, err := ParseDisguiseKey(info.ApiKey)
	if err != nil {
		return err
	}

	switch key.Type {
	case DisguiseKeyTypeSub2API:
		req.Set("Authorization", "Bearer "+key.APIKey)
	case DisguiseKeyTypeOAuth:
		if key.AccessToken == "" {
			return errors.New("codex disguise channel: access_token is required (refresh not supported on hot path)")
		}
		req.Set("Authorization", "Bearer "+key.AccessToken)
		if key.AccountID != "" {
			req.Set("chatgpt-account-id", key.AccountID)
		}
	case DisguiseKeyTypeAgent:
		agentKey, err := agentIdentityKeyFromDisguiseKey(key)
		if err != nil {
			return err
		}
		assertion, err := buildAgentAssertion(agentKey, nowFunc())
		if err != nil {
			return err
		}
		req.Set("Authorization", assertion)
	}

	canonicalUA := buildCodexCLIUserAgent(codexClientVersionFromSettings(info))
	version := codexClientVersionFromSettings(info)
	enforce := codexDisguiseEnforceIdentity(info)

	ensureCodexIdentityHeaders(req, canonicalUA, version)
	a.applyFingerprintHeaders(c, info, req)
	a.guardTurnState(c, info, req)
	enforceCodexIdentityHeaders(req, canonicalUA, version, enforce)

	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	a.captureTurnState(c, info, resp)
	switch info.RelayMode {
	case relayconstant.RelayModeResponsesCompact:
		return openai.OaiResponsesCompactionHandler(c, resp)
	case relayconstant.RelayModeResponses:
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)
	default:
		return nil, types.NewError(errors.New("codex disguise channel: endpoint not supported"), types.ErrorCodeInvalidRequest)
	}
}

// ---- 指纹 / turn-state 辅助（gin context 暂存共享 IDs）----

func (a *Adaptor) stageCodexFingerprintIDs(c *gin.Context, info *relaycommon.RelayInfo) {
	if c == nil || info == nil {
		return
	}
	mode := codexDisguiseFingerprintMode(info)
	seed := codexDisguiseFingerprintSeed(info)
	clientSessionID := ""
	if c.Request != nil {
		clientSessionID = strings.TrimSpace(c.Request.Header.Get("session-id"))
		if clientSessionID == "" {
			clientSessionID = strings.TrimSpace(c.Request.Header.Get("session_id"))
		}
	}
	ids := resolveCodexFingerprintIDs(mode, seed, clientSessionID)
	c.Set(codexFingerprintIDsContextKey, ids)
	if ids != nil {
		c.Set(codexThreadIDContextKey, ids.ThreadID)
	}
}

func (a *Adaptor) stagedCodexFingerprintIDs(c *gin.Context) *codexFingerprintIDs {
	if c == nil {
		return nil
	}
	value, ok := c.Get(codexFingerprintIDsContextKey)
	if !ok {
		return nil
	}
	ids, _ := value.(*codexFingerprintIDs)
	return ids
}

func (a *Adaptor) stagedThreadID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, _ := c.Get(codexThreadIDContextKey)
	threadID, _ := value.(string)
	return threadID
}

func (a *Adaptor) applyFingerprintHeaders(c *gin.Context, info *relaycommon.RelayInfo, req *http.Header) {
	if c == nil || req == nil {
		return
	}
	if body, ok := c.Get(codexTurnStateBlobContextKey); ok {
		if blob, ok := body.(string); ok && blob != "" {
			req.Set("x-codex-turn-state", blob)
			return
		}
	}
	applyCodexFingerprintHeaders(req, a.stagedCodexFingerprintIDs(c))
}

func (a *Adaptor) guardTurnState(c *gin.Context, info *relaycommon.RelayInfo, req *http.Header) {
	if c == nil || req == nil || c.Request == nil {
		return
	}
	blob := strings.TrimSpace(c.Request.Header.Get("x-codex-turn-state"))
	if blob == "" {
		return
	}
	threadID := a.stagedThreadID(c)
	guarded := turnStates.guard(threadID, blob)
	if guarded == "" {
		req.Del("x-codex-turn-state")
		return
	}
	req.Set("x-codex-turn-state", guarded)
}

func (a *Adaptor) captureTurnState(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) {
	if c == nil || resp == nil {
		return
	}
	blob := strings.TrimSpace(resp.Header.Get("x-codex-turn-state"))
	if blob == "" {
		return
	}
	threadID := a.stagedThreadID(c)
	turnStates.note(threadID, blob)
	c.Header("x-codex-turn-state", blob)
}

// ---- 渠道配置读取（ChannelOtherSettings，T7 Step 0 新增字段）----

func codexDisguiseFingerprintMode(info *relaycommon.RelayInfo) FingerprintMode {
	if info != nil && info.ChannelOtherSettings != nil && info.ChannelOtherSettings.FingerprintMode != "" {
		return ParseFingerprintMode(info.ChannelOtherSettings.FingerprintMode)
	}
	return FingerprintModeSession
}

func codexDisguiseFingerprintSeed(info *relaycommon.RelayInfo) string {
	if info != nil && info.ChannelOtherSettings != nil {
		if seed := strings.TrimSpace(info.ChannelOtherSettings.FingerprintSeed); seed != "" {
			return seed
		}
	}
	return ""
}

func codexClientVersionFromSettings(info *relaycommon.RelayInfo) string {
	if info != nil && info.ChannelOtherSettings != nil {
		if v := NormalizeCodexClientVersion(info.ChannelOtherSettings.CodexClientVersion); v != "" {
			return v
		}
	}
	return codexCLIVersion
}

func codexDisguiseEnforceIdentity(info *relaycommon.RelayInfo) bool {
	if info != nil && info.ChannelOtherSettings != nil && info.ChannelOtherSettings.EnforceIdentity != nil {
		return *info.ChannelOtherSettings.EnforceIdentity
	}
	return true
}

var nowFunc = time.Now
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./relay/channel/codexdisguise/ -run 'TestGetRequestURL|TestConvertOpenAIResponses|TestSetupRequestHeader' -v`
Expected: 全部 PASS

**注**：`info.ChannelOtherSettings` 是 `*dto.ChannelOtherSettings`（指针），T7 Step 0 已新增强类型字段，无需断言。`ModelList`/`ChannelName` 常量在 Task 7 的 `constants.go` 中定义：

```go
package codexdisguise

var ModelList = []string{
	"gpt-5-codex",
	"gpt-5.1-codex",
	"gpt-5.2-codex",
	"gpt-5.3-codex",
	"gpt-5.4-codex",
	"gpt-5.5-codex",
	"codex-mini",
	"codex-auto-review",
}

const ChannelName = "codexdisguise"
```

- [ ] **Step 5: Commit**

```bash
git add relay/channel/codexdisguise/adaptor.go relay/channel/codexdisguise/adaptor_test.go relay/channel/codexdisguise/constants.go relaykit/dto/channel_settings.go
git commit -m "feat(codex-disguise): add disguise adaptor wiring and channel settings"
```

---

### Task 8: 路由组 /backend-api/codex

**Files:**
- Modify: `router/relay-router.go`
- Test: `router/relay_router_test.go`（追加）

**Interfaces:**
- Consumes: `controller.Relay`、`types.RelayFormatOpenAIResponses`/`RelayFormatOpenAIResponsesCompaction`/`RelayFormatOpenAIAlphaSearch`
- Produces: `/backend-api/codex/responses`、`/responses/compact`、`/alpha/search`、`/models` 入站路由（挂 TokenAuth + Distribute）

- [ ] **Step 1: 写失败测试**

在 `router/relay_router_test.go` 追加（先确认该文件现有测试风格，若不存在则新建 `router/codex_disguise_router_test.go`）：

```go
package router

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexDisguiseRoutesRegisterWithoutConflict(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/backend-api/codex/responses", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	// 未带 token 应返回 401（而非 404），证明路由已挂载
	assert.Equal(t, 401, w.Code)

	req = httptest.NewRequest("POST", "/backend-api/codex/responses/compact", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)

	req = httptest.NewRequest("POST", "/backend-api/codex/alpha/search", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./router/ -run TestCodexDisguiseRoutesRegisterWithoutConflict -v`
Expected: FAIL（404，路由未挂载）

- [ ] **Step 3: 实现路由**

`router/relay-router.go` 在 `relayV1Router` 组之后新增（复用同一中间件链模式）：

```go
	codexDisguiseRouter := router.Group("/backend-api/codex")
	codexDisguiseRouter.Use(middleware.RouteTag("relay"))
	codexDisguiseRouter.Use(middleware.SystemPerformanceCheck())
	codexDisguiseRouter.Use(middleware.TokenAuth())
	codexDisguiseRouter.Use(middleware.ModelRequestRateLimit())
	{
		httpRouter := codexDisguiseRouter.Group("")
		httpRouter.Use(middleware.Distribute())

		httpRouter.POST("/responses", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponses)
		})
		httpRouter.POST("/responses/compact", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		})
		httpRouter.POST("/alpha/search", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAlphaSearch)
		})
		httpRouter.GET("/models", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeCodexDisguise)
		})
	}
```

`controller.ListModels` 需要支持 `ChannelTypeCodexDisguise`（校验现有实现：若按 ChannelType 分发模型列表则补 case；模型列表复用 codexdisguise.ModelList）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./router/ -run TestCodexDisguiseRoutesRegisterWithoutConflict -v`
Expected: PASS（401 而非 404）

- [ ] **Step 5: 全量编译验证**

Run: `go build ./... && go vet ./relay/channel/codexdisguise/ ./router/`
Expected: 无错误

- [ ] **Step 6: Commit**

```bash
git add router/relay-router.go router/codex_disguise_router_test.go
git commit -m "feat(codex-disguise): expose /backend-api/codex ingress routes"
```

---

### Task 9: 版本同步服务（service/codex_disguise_version_sync.go）

**Files:**
- Create: `service/codex_disguise_version_sync.go`
- Create: `service/codex_disguise_version_sync_test.go`

**Interfaces:**
- Consumes（主模块已有）: `service.GetLatestCodexClientVersion(ctx, client)`（GitHub latest release，1h 缓存）
- Produces:
  - `func GetCodexDisguiseClientVersion(ctx context.Context, proxyURL string) (string, error)` — 优先读全局 Option `CodexDisguiseClientVersion`（管理员手配），空则 `GetLatestCodexClientVersion`
  - `func SetCodexDisguiseClientVersion(version string) error` — `model.UpdateOption("CodexDisguiseClientVersion", version)` + `model.InitOptionMap()`

- [ ] **Step 1: 写失败测试**

```go
package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCodexDisguiseClientVersionPrefersOption(t *testing.T) {
	// 通过 UpdateOption 注入手配值
	require.NoError(t, SetCodexDisguiseClientVersion("0.147.0"))
	version, err := GetCodexDisguiseClientVersion(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "0.147.0", version)
}

func TestGetCodexDisguiseClientVersionFallsBackToGitHub(t *testing.T) {
	require.NoError(t, SetCodexDisguiseClientVersion(""))
	version, err := GetCodexDisguiseClientVersion(context.Background(), "")
	require.NoError(t, err)
	assert.NotEmpty(t, version)
}

func TestGetCodexDisguiseClientVersionRejectsInvalid(t *testing.T) {
	err := SetCodexDisguiseClientVersion("not-a-version; rm -rf")
	require.Error(t, err)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./service/ -run TestGetCodexDisguiseClientVersion -v`
Expected: 编译失败（函数未定义）

- [ ] **Step 3: 实现**

```go
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// CodexDisguiseClientVersionOptionKey 管理员手配的 Codex 伪装出站版本。
// 空值 = 使用 GitHub 自动同步的最新稳定版。
const CodexDisguiseClientVersionOptionKey = "CodexDisguiseClientVersion"

var codexDisguiseVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}(-[0-9A-Za-z.]+)?$`)

func GetCodexDisguiseClientVersion(ctx context.Context, proxyURL string) (string, error) {
	if v := strings.TrimSpace(common.OptionMap[CodexDisguiseClientVersionOptionKey]); v != "" {
		if codexDisguiseVersionPattern.MatchString(v) {
			return v, nil
		}
		return "", errors.New("codex disguise client version option is invalid")
	}
	client, err := GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return "", err
	}
	return GetLatestCodexClientVersion(ctx, client)
}

func SetCodexDisguiseClientVersion(version string) error {
	version = strings.TrimSpace(version)
	if version != "" && !codexDisguiseVersionPattern.MatchString(version) {
		return errors.New("codex disguise client version must match X.Y.Z or X.Y.Z-alpha.N")
	}
	if err := model.UpdateOption(CodexDisguiseClientVersionOptionKey, version); err != nil {
		return err
	}
	model.InitOptionMap()
	return nil
}
```

**注**：`GetHttpClientWithProxy` 若不存在则以 `http.DefaultClient` 代替（检查 `service` 包现有代理客户端工厂）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./service/ -run TestGetCodexDisguiseClientVersion -v`
Expected: 全部 PASS（GitHub 网络依赖的用例若环境无网，用 httptest stub 注入 `codexLatestReleaseURL`——检查 `service/codex_models.go` 是否暴露可替换 URL 变量，若为 const 则改为 var 或在测试中允许失败断言）

- [ ] **Step 5: Commit**

```bash
git add service/codex_disguise_version_sync.go service/codex_disguise_version_sync_test.go
git commit -m "feat(codex-disguise): add client version sync service"
```

---

### Task 10: 全量验证与收尾

**Files:**
- Modify: 视验证结果修复

- [ ] **Step 1: 后端全量测试**

Run: `go build ./... && go test ./relay/channel/codexdisguise/ ./service/ -count=1`
Expected: 全部 PASS

- [ ] **Step 2: relaykit 独立构建验证**

Run: `cd relaykit && GOWORK=off go build ./...`
Expected: 成功（本计划未涉及 relaykit，仅验证无回归）

- [ ] **Step 3: 前端 typecheck + lint**

Run: `bun run typecheck`（在 `web/` 下）与 `bun run lint`
Expected: 无 error（涉及 channels constants 改动）

- [ ] **Step 4: 回归冒烟**

Run: `go test ./router/ ./constant/ ./relay/ -count=1`
Expected: 全部 PASS

- [ ] **Step 5: Commit 收尾**

```bash
git add -A
git commit -m "chore(codex-disguise): verification pass"
```

---

## Self-Review 记录

- **Spec 覆盖**：渠道类型（T1）、Key 三格式（T2）、身份三元组（T3）、指纹收敛（T4）、turn-state（T5）、Agent Identity（T6）、适配器接线（T7）、对外 codex 路由（T8）、版本同步（T9）、错误处理与测试（各任务内置）、前端补位（T1）、计费/多 Key/代理复用（沿用现有机制，无新代码）。
- **已知偏差**：OAuth 热路径不做自动刷新重试（与主模块现有 codex 渠道一致，刷新仅存在于管理端操作与 Task 9 之外的管理界面）；`instructions` 注入复用现有 codex 渠道语义（缺省 `""`），不内嵌 sub2api 的 base prompt 大文本（主模块无该资源）。
- **relaykit 约束**：`ChannelOtherSettings` 位于 relaykit 模块，T7 Step 0 扩展后必须执行 `cd relaykit && GOWORK=off go build ./...`（已写入任务步骤）。
- **指纹 seed 缺省**：适配器在 seed 为空时返回 ""（不收敛），`FingerprintMode` 缺省 session 但无 seed 时不改写——管理员须在渠道配置 `fingerprint_seed` 填入稳定 UUIDv4（渠道创建时可自动生成，前端表单预留字段，本期不做自动生成）。
- **类型一致性**：`DisguiseKey`/`codexFingerprintIDs`/`codexOutboundIdentity`/`turnStateRegistry`/`agentIdentityKey` 在各任务间签名一致；`FingerprintMode` 与配置键 `fingerprint_mode` 映射一致。
- **依赖顺序**：T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10；T7 依赖 T2-T6 全部符号。