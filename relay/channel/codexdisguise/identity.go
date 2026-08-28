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
func enforceCodexIdentityHeaders(h *http.Header, canonicalUA, version string, enforce bool) {
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

func pairCodexIdentityHeaders(h *http.Header, identity codexOutboundIdentity) {
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
func ensureCodexIdentityHeaders(h *http.Header, canonicalUA, version string) {
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