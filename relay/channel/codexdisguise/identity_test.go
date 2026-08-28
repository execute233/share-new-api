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
	// 两段/三段数字均合法（与 sub2api 正则语义一致）
	assert.Equal(t, "1.2", NormalizeCodexClientVersion("1.2"))
	assert.Empty(t, NormalizeCodexClientVersion(""))
	assert.Empty(t, NormalizeCodexClientVersion("v1.2"))
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
	// 任意 {client}/{version} 形态都会重建版本段（与 sub2api 语义一致）
	assert.Equal(t, "curl/0.146.0", setCodexUserAgentVersion("curl/8.0", "0.146.0"))
	// 无 '/' 或 `client/` 无版本段 → 不可重建，返回空串
	assert.Empty(t, setCodexUserAgentVersion("curl", "0.146.0"))
	assert.Empty(t, setCodexUserAgentVersion("curl/", "0.146.0"))
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
	h := &http.Header{}
	h.Set("originator", "evil")
	h.Set("user-agent", "evil/1.0")
	h.Set("version", "0.1.0")
	enforceCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0", true)
	assert.Equal(t, "codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color", h.Get("user-agent"))
	assert.Equal(t, "codex-tui", h.Get("originator"))
	assert.Equal(t, "0.146.0", h.Get("version"))
}

func TestEnforceCodexIdentityHeadersSkipsWithoutOriginator(t *testing.T) {
	h := &http.Header{}
	h.Set("user-agent", "evil/1.0")
	enforceCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0", true)
	assert.Equal(t, "evil/1.0", h.Get("user-agent"), "无 originator 的请求不被补回身份")
}

func TestEnforceCodexIdentityHeadersDisabledPairsOnly(t *testing.T) {
	h := &http.Header{}
	h.Set("originator", "codex_vscode")
	h.Set("user-agent", "codex_vscode/1.0.0 (Ubuntu 22.4.0; x86_64)")
	h.Set("version", "0.1.0")
	enforceCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0", false)
	assert.Equal(t, "codex_vscode", h.Get("originator"), "关闭强制统一保留客户端身份")
	assert.Equal(t, "0.146.0", h.Get("version"), "版本仍被门槛校正")
}

func TestEnsureCodexIdentityHeaders(t *testing.T) {
	h := &http.Header{}
	ensureCodexIdentityHeaders(h, buildCodexCLIUserAgent("0.146.0"), "0.146.0")
	assert.Equal(t, buildCodexCLIUserAgent("0.146.0"), h.Get("user-agent"))
	assert.Equal(t, "codex-tui", h.Get("originator"))
	assert.Equal(t, "0.146.0", h.Get("version"))
	assert.Equal(t, "responses=experimental", h.Get("OpenAI-Beta"))
}
