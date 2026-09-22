# Shadowsocks 代理支持 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为代理池与 channel 级 proxy_url 新增 `ss://` shadowsocks 协议支持（AEAD 加密，复用现有 username/password 承载）。

**Architecture:** URL 解析在 `common/proxy_url.go`（新增 ss scheme + userinfo 明文/base64 变形归一化），协议白名单在 `model/proxy.go`，传输层新增 `service/shadowsocks_dialer.go`（go-shadowsocks2 `core.PickCipher` + 手写 SOCKS5 CONNECT），接线在 `service/http_client.go` 的 `configureProxyTransport`。前端 `proxy-mutate-drawer.tsx` 加 `ss` 选项与字段标注切换。

**Tech Stack:** Go 1.22+, go-shadowsocks2 v0.1.5（core/socks 包，仅 AEAD），golang.org/x/net（已有），React 19 + Base UI + i18next。

## Global Constraints

- 凭据承载：method 入 `username`、password 入 `password`（零表迁移、零 API 字段改动）
- AEAD 白名单（v0.1.5 实际支持，大小写不敏感）：`aes-128-gcm`（别名 `AEAD_AES_128_GCM`）、`aes-256-gcm`（`AEAD_AES_256_GCM`）、`chacha20-ietf-poly1305`（`AEAD_CHACHA20_POLY1305`）。**没有** aes-192-gcm / xchacha20 / stream ciphers；`DUMMY` 永不开放
- URL 变体：明文 `ss://method:password@host:port` 与 `ss://base64url(method:password)@host:port`（SIP002）
- 端口缺省 **8388**；域名在加密隧道内原样发送（远端 DNS，语义同 socks5h）
- 密码含 URL 保留字符（`#:?@/`）时明文解析报错，提示改用 base64 变形
- 作用范围：代理池（Proxy 表）与 channel 级 `proxy_url` 两处都支持（同一路径）
- 设计文档 `docs/superpowers/specs/2026-08-27-shadowsocks-proxy-design.md` 与计划文档**均不 git 提交**
- 代码提交：每任务结束 git commit（若 gpg 弹窗超时则加 `--no-gpg-sign`）
- relaykit 模块不得受影响：`cd relaykit && $env:GOWORK='off'; go build ./...` 必须通过
- 所有 JSON 操作走 `common.Marshal/Unmarshal`（本计划不涉及 JSON 序列化）
- 前端 i18n：locale JSON 禁止手改，走既有 `i18n:sync` 流程（实施前加载 i18n-translate skill）

---

### Task 1: common/proxy_url.go — ss scheme 解析与 userinfo 归一化

**Files:**
- Modify: `common/proxy_url.go`（parseProxyURL 内 scheme switch、端口缺省、新增 3 个私有函数）
- Create: `common/proxy_url_test.go`

**Interfaces:**
- Consumes: 无（纯 URL 解析）
- Produces: `parseProxyURL(raw string, allowLegacySuffix bool) (*url.URL, bool, error)` 行为扩展——`ss` scheme 被接受；返回的 `parsedURL.User` 为 `url.UserPassword(canonicalMethod, password)`（明文归一化）；ss 无端口时 `parsedURL.Host` 注入 `:8388`
- 归一化规则：userinfo 含 `:` → 明文 `method:password`；不含 `:` → 整个 userinfo 按 base64url 解码（先 `base64.RawURLEncoding` 后 `base64.URLEncoding`），解码后按第一个 `:` 切分

- [ ] **Step 1: 写失败测试** `common/proxy_url_test.go`

```go
package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProxyURLStrictShadowsocks(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		username string
		password string
		host     string
		port     string
	}{
		{
			name:     "plain userinfo",
			raw:      "ss://chacha20-ietf-poly1305:pwd@proxy.example.com:8388",
			username: "chacha20-ietf-poly1305",
			password: "pwd",
			host:     "proxy.example.com",
			port:     "8388",
		},
		{
			name:     "uppercase alias normalized",
			raw:      "ss://AEAD_AES_256_GCM:secret@proxy.example.com:9000",
			username: "aes-256-gcm",
			password: "secret",
			host:     "proxy.example.com",
			port:     "9000",
		},
		{
			name:     "default port 8388",
			raw:      "ss://aes-256-gcm:secret@proxy.example.com",
			username: "aes-256-gcm",
			password: "secret",
			host:     "proxy.example.com",
			port:     "8388",
		},
		{
			name:     "empty password with colon",
			raw:      "ss://aes-128-gcm:@proxy.example.com",
			username: "aes-128-gcm",
			password: "",
			host:     "proxy.example.com",
			port:     "8388",
		},
		{
			name:     "password containing colon",
			raw:      "ss://aes-256-gcm:pa:ss@proxy.example.com:8388",
			username: "aes-256-gcm",
			password: "pa:ss",
			host:     "proxy.example.com",
			port:     "8388",
		},
		{
			name:     "base64url variant",
			raw:      "ss://YWVzLTI1Ni1nY206cHdk@proxy.example.com:8388",
			username: "aes-256-gcm",
			password: "pwd",
			host:     "proxy.example.com",
			port:     "8388",
		},
		{
			name:     "base64url with padding",
			raw:      "ss://YWVzLTI1Ni1nY206cA==@proxy.example.com:8388",
			username: "aes-256-gcm",
			password: "p",
			host:     "proxy.example.com",
			port:     "8388",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseProxyURLStrict(tt.raw)
			require.NoError(t, err)
			require.NotNil(t, parsed)
			assert.Equal(t, "ss", parsed.Scheme)
			assert.Equal(t, tt.username, parsed.User.Username())
			password, _ := parsed.User.Password()
			assert.Equal(t, tt.password, password)
			assert.Equal(t, tt.host, parsed.Hostname())
			assert.Equal(t, tt.port, parsed.Port())
		})
	}
}

func TestParseProxyURLStrictShadowsocksRejects(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "unknown method",
			raw:     "ss://rc4-md5:pwd@proxy.example.com:8388",
			wantErr: "unsupported shadowsocks encryption method",
		},
		{
			name:    "invalid base64",
			raw:     "ss://!!!not-base64@proxy.example.com:8388",
			wantErr: "invalid shadowsocks proxy URL credentials",
		},
		{
			name:    "base64 decodes without colon",
			raw:     "ss://YQ@proxy.example.com:8388",
			wantErr: "invalid shadowsocks proxy URL credentials",
		},
		{
			name:    "missing host",
			raw:     "ss://aes-256-gcm:pwd@",
			wantErr: "proxy URL must include a host",
		},
		{
			name:    "no userinfo at all",
			raw:     "ss://proxy.example.com:8388",
			wantErr: "shadowsocks proxy URL must include method and password",
		},
		{
			name:    "password with hash treated as fragment in strict mode",
			raw:     "ss://aes-256-gcm:pa#ss@proxy.example.com:8388",
			wantErr: "must not include a fragment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseProxyURLStrict(tt.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, parsed)
		})
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./common/ -run TestParseProxyURLStrictShadowsocks -count=1 -v`
Expected: FAIL（`unsupported proxy scheme` 或编译失败）

- [ ] **Step 3: 实现** `common/proxy_url.go`

在 `parseProxyURL` 中修改 scheme 白名单与端口缺省，并新增三个函数：

```go
var supportedShadowsocksMethods = map[string]string{
	"AES-128-GCM":            "aes-128-gcm",
	"AEAD_AES_128_GCM":       "aes-128-gcm",
	"AES-256-GCM":            "aes-256-gcm",
	"AEAD_AES_256_GCM":       "aes-256-gcm",
	"CHACHA20-IETF-POLY1305": "chacha20-ietf-poly1305",
	"AEAD_CHACHA20_POLY1305": "chacha20-ietf-poly1305",
}

func normalizeShadowsocksMethod(method string) (string, error) {
	canonical, ok := supportedShadowsocksMethods[strings.ToUpper(strings.TrimSpace(method))]
	if !ok {
		return "", fmt.Errorf("unsupported shadowsocks encryption method: %s", method)
	}
	return canonical, nil
}

func normalizeShadowsocksUserInfo(parsedURL *url.URL) error {
	if parsedURL.User == nil || parsedURL.User.String() == "" {
		return fmt.Errorf("shadowsocks proxy URL must include method and password")
	}
	userinfo := parsedURL.User.String()
	method, password := "", ""
	if strings.Contains(userinfo, ":") {
		method = parsedURL.User.Username()
		password, _ = parsedURL.User.Password()
	} else {
		decoded, err := decodeShadowsocksUserInfo(userinfo)
		if err != nil {
			return err
		}
		parts := strings.SplitN(decoded, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return fmt.Errorf("invalid shadowsocks proxy URL credentials; use method:password or base64url(method:password)")
		}
		method, password = parts[0], parts[1]
	}
	canonical, err := normalizeShadowsocksMethod(method)
	if err != nil {
		return err
	}
	parsedURL.User = url.UserPassword(canonical, password)
	return nil
}

func decodeShadowsocksUserInfo(encoded string) (string, error) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding} {
		if decoded, err := enc.DecodeString(encoded); err == nil {
			return string(decoded), nil
		}
	}
	return "", fmt.Errorf("invalid shadowsocks proxy URL credentials; use method:password or base64url(method:password)")
}
```

`parseProxyURL` 内两处修改：

```go
	switch parsedURL.Scheme {
	case "http", "https", "socks5", "socks5h", "ss":
	default:
		return nil, false, fmt.Errorf("proxy URL must use http, https, socks5, socks5h, or ss")
	}
```

scheme switch 之后、hostname 检查之前：

```go
	if parsedURL.Scheme == "ss" {
		if err := normalizeShadowsocksUserInfo(parsedURL); err != nil {
			return nil, false, err
		}
	}
```

端口缺省段（socks 分支旁）：

```go
	if parsedURL.Scheme == "ss" && parsedURL.Port() == "" {
		parsedURL.Host = net.JoinHostPort(parsedURL.Hostname(), "8388")
	}
```

import 增加 `"encoding/base64"`。

注意：`no userinfo at all` 用例中 `ss://proxy.example.com:8388` 的 `parsedURL.User` 为 nil（Go url.Parse 对无 @ 的 URL 不设 User）——所以 `normalizeShadowsocksUserInfo` 的 nil 检查用 `parsedURL.User == nil || parsedURL.User.String() == ""` 双保险（`ss://@host` 形式 User 非 nil 但 String 为空串）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./common/ -count=1`
Expected: PASS（含原有 common 测试回归）

- [ ] **Step 5: 提交**

```bash
git add common/proxy_url.go common/proxy_url_test.go
git commit -m "feat(proxy): parse shadowsocks ss:// proxy URLs with AEAD method validation"
```

---

### Task 2: model/proxy.go — 协议白名单与批量导入支持

**Files:**
- Modify: `model/proxy.go`（常量 + IsSupportedProxyProtocol）
- Modify: `model/proxy_test.go`（追加用例）
- Modify: `controller/proxy.go`（sanitizeProxyInputError 透传 shadowsocks 错误）

**Interfaces:**
- Consumes: `common.ParseProxyURLStrict`（Task 1，已支持 ss）
- Produces: `model.ProxyProtocolShadowsocks = "ss"`；`IsSupportedProxyProtocol("ss") == true`；`ParseProxyEndpoint` 对 ss URL 返回 method/username、password、host、port（缺省 8388 由 strict 解析注入，无需改代码——仅补测试）

- [ ] **Step 1: 写失败测试**（追加到 `model/proxy_test.go`）

```go
func TestProxyProtocolShadowsocks(t *testing.T) {
	assert.True(t, IsSupportedProxyProtocol("ss"))
	assert.True(t, IsSupportedProxyProtocol("SS"))

	protocol, host, port, username, password, err := ParseProxyEndpoint("ss://chacha20-ietf-poly1305:secret@proxy.example.com")
	require.NoError(t, err)
	assert.Equal(t, "ss", protocol)
	assert.Equal(t, "proxy.example.com", host)
	assert.Equal(t, 8388, port)
	assert.Equal(t, "chacha20-ietf-poly1305", username)
	assert.Equal(t, "secret", password)

	_, _, _, _, _, err = ParseProxyEndpoint("ss://rc4-md5:secret@proxy.example.com:8388")
	require.Error(t, err)
}
```

（先确认 `model/proxy_test.go` 现有 import；若缺 `require` 则补 `"github.com/stretchr/testify/require"`）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./model/ -run TestProxyProtocolShadowsocks -count=1 -v`
Expected: FAIL（`IsSupportedProxyProtocol` 返回 false）

- [ ] **Step 3: 实现**

`model/proxy.go` 常量块追加：

```go
	ProxyProtocolShadowsocks = "ss"
```

`IsSupportedProxyProtocol` switch 追加：

```go
	case ProxyProtocolShadowsocks:
		return true
```

`controller/proxy.go` 的 `sanitizeProxyInputError` 中把 shadowsocks 专属错误信息透传给用户（否则被折叠成 "invalid proxy configuration"）：

```go
	if strings.Contains(message, "stable") || strings.HasPrefix(message, "proxy URL") || strings.Contains(message, "shadowsocks") {
		return message
	}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./model/ -count=1` 与 `go test ./controller/ -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add model/proxy.go model/proxy_test.go controller/proxy.go
git commit -m "feat(proxy): register shadowsocks protocol in model validation and quick-add parsing"
```

---

### Task 3: service/shadowsocks_dialer.go — SS 拨号器 + 依赖

**Files:**
- Modify: `go.mod` / `go.sum`（go get）
- Create: `service/shadowsocks_dialer.go`
- Create: `service/shadowsocks_dialer_test.go`（集成测试：真实 SS 服务器回环）

**Interfaces:**
- Consumes: `common.ParseProxyURLStrict` 归一化后的 userinfo（Task 1）
- Produces:
  - `func newShadowsocksDialer(method, password string, host string, port int, forwardDialer *net.Dialer) (*shadowsocksDialer, error)` —— method 非法时返回错误（`core.PickCipher` 报错）
  - `(*shadowsocksDialer).DialContext(ctx context.Context, network, address string) (net.Conn, error)` —— 实现 `proxy.ContextDialer`；`address` 为 `host:port` 目标（域名原样经加密隧道发送）

- [ ] **Step 1: 添加依赖**

```bash
go get github.com/shadowsocks/go-shadowsocks2@v0.1.5
go mod tidy
```

Expected: go.mod 增加 `github.com/shadowsocks/go-shadowsocks2 v0.1.5`（间接依赖 `github.com/riobard/go-bloom`、`golang.org/x/crypto` 进 go.sum）

- [ ] **Step 2: 写失败测试** `service/shadowsocks_dialer_test.go`

```go
package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSSMethod = "chacha20-ietf-poly1305"
const testSSPassword = "test-secret-password"

type ssProbe struct {
	targets chan string
}

func startTestShadowsocksServer(t *testing.T, method, password string, probe *ssProbe) string {
	t.Helper()
	cipher, err := core.PickCipher(method, nil, password)
	require.NoError(t, err)
	listener, err := core.Listen("tcp", "127.0.0.1:0", cipher)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				target, err := socks.Handshake(conn)
				if err != nil {
					_ = conn.Close()
					return
				}
				if probe != nil {
					probe.targets <- target.String()
				}
				upstream, err := net.Dial("tcp", target.String())
				if err != nil {
					_ = conn.Close()
					return
				}
				go func() { _, _ = io.Copy(upstream, conn); _ = upstream.Close() }()
				_, _ = io.Copy(conn, upstream)
				_ = conn.Close()
			}()
		}
	}()
	return listener.Addr().String()
}

func newTestDialer(t *testing.T, ssAddr string, password string) *shadowsocksDialer {
	t.Helper()
	host, portText, err := net.SplitHostPort(ssAddr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	dialer, err := newShadowsocksDialer(testSSMethod, password, host, port, &net.Dialer{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return dialer
}

func TestShadowsocksDialerReachesHTTPServer(t *testing.T) {
	ssAddr := startTestShadowsocksServer(t, testSSMethod, testSSPassword, nil)
	hit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Write([]byte("via-shadowsocks"))
	}))
	defer upstream.Close()

	dialer := newTestDialer(t, ssAddr, testSSPassword)
	client := &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}
	resp, err := client.Get(upstream.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "via-shadowsocks", string(body))
	assert.True(t, hit, "upstream server must receive the request")
}

func TestShadowsocksDialerPreservesDomainName(t *testing.T) {
	probe := &ssProbe{targets: make(chan string, 1)}
	ssAddr := startTestShadowsocksServer(t, testSSMethod, testSSPassword, probe)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	_, portText, err := net.SplitHostPort(upstream.URL[len("http://"):])
	require.NoError(t, err)

	dialer := newTestDialer(t, ssAddr, testSSPassword)
	conn, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort("localhost", portText))
	require.NoError(t, err)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
	require.NoError(t, err)

	select {
	case target := <-probe.targets:
		assert.True(t, strings.HasPrefix(target, "localhost:"), "target must keep domain form, got %q", target)
	case <-time.After(5 * time.Second):
		t.Fatal("ss server never received a handshake")
	}
}

func TestShadowsocksDialerRejectsWrongPassword(t *testing.T) {
	ssAddr := startTestShadowsocksServer(t, testSSMethod, testSSPassword, nil)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	dialer := newTestDialer(t, ssAddr, "wrong-password")
	client := &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}
	resp, err := client.Get(upstream.URL)
	if err == nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err, "wrong password must fail the encrypted handshake")
}

func TestShadowsocksDialerContextCancellation(t *testing.T) {
	ssAddr := startTestShadowsocksServer(t, testSSMethod, testSSPassword, nil)
	dialer := newTestDialer(t, ssAddr, testSSPassword)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := dialer.DialContext(ctx, "tcp", "example.com:80")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestNewShadowsocksDialerRejectsUnsupportedCipher(t *testing.T) {
	_, err := newShadowsocksDialer("rc4-md5", "pwd", "proxy.example.com", 8388, &net.Dialer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cipher")
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./service/ -run TestShadowsocksDialer -count=1 -v`
Expected: FAIL（编译失败：`newShadowsocksDialer` 未定义）

- [ ] **Step 4: 实现** `service/shadowsocks_dialer.go`

```go
package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
)

// shadowsocksDialer 建立到 Shadowsocks 服务器的加密 TCP 隧道，并在隧道内完成
// SOCKS5 CONNECT 握手。目标地址以域名原样发送，由 SS 服务端解析（远端 DNS）。
type shadowsocksDialer struct {
	cipher    core.Cipher
	ssAddress string
	forward   *net.Dialer
}

func newShadowsocksDialer(method, password string, host string, port int, forwardDialer *net.Dialer) (*shadowsocksDialer, error) {
	cipher, err := core.PickCipher(method, nil, password)
	if err != nil {
		return nil, fmt.Errorf("shadowsocks: %w", err)
	}
	return &shadowsocksDialer{
		cipher:    cipher,
		ssAddress: net.JoinHostPort(host, strconv.Itoa(port)),
		forward:   forwardDialer,
	}, nil
}

func (d *shadowsocksDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("shadowsocks: unsupported network %q", network)
	}
	target := socks.ParseAddr(address)
	if target == nil {
		return nil, fmt.Errorf("shadowsocks: invalid target address %q", address)
	}
	rawConn, err := d.forward.DialContext(ctx, "tcp", d.ssAddress)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = rawConn.SetDeadline(deadline)
	}
	conn := d.cipher.StreamConn(rawConn)
	if err := shadowsocksConnect(conn, target); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// shadowsocksConnect 在加密隧道内发送 SOCKS5 CONNECT 请求并校验回复。
// go-shadowsocks2 服务端对 CONNECT 固定回复 10 字节（IPv4 绑地址）。
func shadowsocksConnect(conn net.Conn, target socks.Addr) error {
	request := make([]byte, 0, 1+1+1+len(target))
	request = append(request, 5, socks.CmdConnect, 0)
	request = append(request, target...)
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("shadowsocks: send connect request: %w", err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("shadowsocks: read connect reply: %w", err)
	}
	if reply[0] != 5 || reply[1] != 0 {
		return fmt.Errorf("shadowsocks: connect failed: %v", socks.Error(reply[1]))
	}
	return nil
}
```

- [ ] **Step 5: 运行确认通过**

Run: `go test ./service/ -run TestShadowsocksDialer -count=1 -v`
Expected: 5 个测试全 PASS（集成测试本地起真实 SS 服务器回环，无外部网络）

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum service/shadowsocks_dialer.go service/shadowsocks_dialer_test.go
git commit -m "feat(proxy): add shadowsocks context dialer with real-server integration tests"
```

---

### Task 4: service/http_client.go — configureProxyTransport 接线 ss

**Files:**
- Modify: `service/http_client.go`（configureProxyTransport 加 case）
- Create: `service/http_client_test.go`

**Interfaces:**
- Consumes: `newShadowsocksDialer`（Task 3）
- Produces: `configureProxyTransport(transport *http.Transport, proxyURL *url.URL) error` 对 `ss` scheme 设置 `transport.DialContext` 且 `transport.Proxy = nil`；非法 method 返回错误

- [ ] **Step 1: 写失败测试** `service/http_client_test.go`

```go
package service

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureProxyTransportShadowsocks(t *testing.T) {
	parsed, err := common.ParseProxyURLStrict("ss://aes-256-gcm:secret@proxy.example.com:8388")
	require.NoError(t, err)

	transport := newRelayHTTPTransport()
	require.NoError(t, configureProxyTransport(transport, parsed))
	assert.Nil(t, transport.Proxy, "ss must not use http.Proxy")
	assert.NotNil(t, transport.DialContext, "ss must install a DialContext")

	badURL, err := url.Parse("ss://rc4-md5:secret@proxy.example.com:8388")
	require.NoError(t, err)
	badTransport := newRelayHTTPTransport()
	require.Error(t, configureProxyTransport(badTransport, badURL))
}
```

（`newRelayHTTPTransport` 为同包私有函数；`common.ParseProxyURLStrict` 对非法 method 报错，故 badURL 直接用 `url.Parse` 绕过解析层）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./service/ -run TestConfigureProxyTransportShadowsocks -count=1 -v`
Expected: FAIL（`configureProxyTransport` 对 ss 返回 "unsupported proxy scheme"）

- [ ] **Step 3: 实现**

`configureProxyTransport` 的 switch 中，socks5 分支后新增：

```go
	case "ss":
		transport.Proxy = nil
		forwardDialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		method := proxyURL.User.Username()
		if method == "" {
			return fmt.Errorf("shadowsocks: proxy URL must include an encryption method")
		}
		password, _ := proxyURL.User.Password()
		port, err := strconv.Atoi(proxyURL.Port())
		if err != nil {
			return fmt.Errorf("shadowsocks: proxy URL must include a valid port")
		}
		dialer, err := newShadowsocksDialer(method, password, proxyURL.Hostname(), port, forwardDialer)
		if err != nil {
			return err
		}
		transport.DialContext = dialer.DialContext
		return nil
```

（`strconv` 已 import；`proxyURL.Port()` 在 Task 1 解析后非空）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./service/ -run TestConfigureProxyTransportShadowsocks -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add service/http_client.go service/http_client_test.go
git commit -m "feat(proxy): wire shadowsocks dialer into channel proxy transport"
```

---

### Task 5: 前端 — drawer 加 ss 选项 + 字段标注 + i18n

**Files:**
- Modify: `web/src/features/proxies/components/proxy-mutate-drawer.tsx`
- Create: `web/src/features/proxies/components/__tests__/proxy-mutate-drawer.test.tsx`
- Modify: `web/src/i18n/locales/en.json`（及 zh/zh-TW/fr/ja/ru/vi，走 i18n 流程）

**Interfaces:**
- Consumes: 无
- Produces: 协议下拉含 `ss`；协议为 `ss` 时 username 字段 label 为 "Encryption method"、placeholder 为 `chacha20-ietf-poly1305`，并显示格式提示文本

- [ ] **Step 1: 写失败测试**

`web/src/features/proxies/components/__tests__/proxy-mutate-drawer.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { ProxyMutateDrawer } from '../proxy-mutate-drawer'

function renderDrawer() {
  return render(
    <ProxyMutateDrawer proxy={null} open onOpenChange={() => {}} onSaved={() => {}} />
  )
}

describe('ProxyMutateDrawer shadowsocks', () => {
  it('lists ss as a selectable protocol', async () => {
    renderDrawer()
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Protocol' }))
    expect(await screen.findByRole('option', { name: 'ss' })).toBeInTheDocument()
  })

  it('switches username field to encryption method when ss is selected', async () => {
    renderDrawer()
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Protocol' }))
    await user.click(await screen.findByRole('option', { name: 'ss' }))
    expect(screen.getByLabelText('Encryption method')).toBeInTheDocument()
    expect(
      screen.getByPlaceholderText('chacha20-ietf-poly1305')
    ).toBeInTheDocument()
  })
})
```

（若现有测试目录/测试风格不同，先看 `web/src/features/` 下既有测试文件的 i18n mock 与 render 辅助约定——`__tests__/` 目录规范见 web/AGENTS.md 3.14。`combobox` 名称需匹配 Select 的可访问名，若 Base UI Select 的可访问名不同则以实际为准调整查询）

- [ ] **Step 2: 运行确认失败**

Run: `bunx vitest run src/features/proxies/components/__tests__/proxy-mutate-drawer.test.tsx`
Expected: FAIL（无 'ss' 选项）

- [ ] **Step 3: 实现** `proxy-mutate-drawer.tsx`

```tsx
const protocols = ['http', 'https', 'socks5', 'socks5h', 'ss']
const isShadowsocks = form.protocol === 'ss'
```

username 字段块改为（label 与 placeholder 条件切换）：

```tsx
<div className='space-y-2'>
  <Label htmlFor='proxy-username'>
    {isShadowsocks ? t('Encryption method') : t('Username')}
  </Label>
  <Input
    id='proxy-username'
    value={form.username}
    onChange={(e) => update('username', e.target.value)}
    placeholder={
      isShadowsocks
        ? 'chacha20-ietf-poly1305'
        : props.proxy?.credential_configured
          ? t('Leave empty to keep current')
          : ''
    }
  />
</div>
```

在 username/password 字段块之后、Status 块之前插入提示（仅 ss 时显示）：

```tsx
{isShadowsocks && (
  <p className='text-sm text-muted-foreground'>
    {t(
      'For Shadowsocks, the encryption method goes in the first field (e.g. aes-256-gcm, chacha20-ietf-poly1305) and the password in the second. You can also paste a full ss:// URL in quick add.'
    )}
  </p>
)}
```

（`text-muted-foreground` 需确认项目 Tailwind 语义色类名，若不同则用项目现有次要文本类）

- [ ] **Step 4: i18n 键补齐（加载 i18n-translate skill 执行）**

新增键（en 源串）：`Encryption method`、提示长句。运行既有同步流程：

```bash
bun run i18n:sync
```

将新键同步至 zh/zh-TW/fr/ja/ru/vi 6 个语言文件（缺失翻译走既有脚本/人工补译流程，遵循 web/AGENTS.md 3.1 与 i18n skill）。

- [ ] **Step 5: 验证**

Run:
```bash
bun run typecheck
bunx vitest run src/features/proxies/components/__tests__/proxy-mutate-drawer.test.tsx
bunx oxlint src/features/proxies/components/proxy-mutate-drawer.tsx
```
Expected: 全通过；`bun run i18n:sync` 报告 missing=0

- [ ] **Step 6: 提交**

```bash
git add web/src/features/proxies web/src/i18n/locales
git commit -m "feat(web): add shadowsocks protocol option with encryption method fields to proxy drawer"
```

---

### Task 6: 文档更新 + 全套验证

**Files:**
- Modify: `docs/spec/outbound-proxy-module.md`（实现决策节：支持格式列表加 `ss`、AEAD 白名单、缺省端口、base64url 变形）
- 说明：设计文档与计划文档不提交（用户要求）

- [ ] **Step 1: 更新文档**

`docs/spec/outbound-proxy-module.md` 的「实现决策」中，将"支持格式 `http`、`https`、`socks5`、`socks5h` URL"改为：

```
支持格式 `http`、`https`、`socks5`、`socks5h`、`ss`（shadowsocks）URL。`ss` 支持
`ss://method:password@host:port` 明文与 `ss://base64url(method:password)@host:port`
变形，method 白名单：aes-128-gcm / aes-256-gcm / chacha20-ietf-poly1305（大小写不敏感），
端口缺省 8388，目标域名经加密隧道原样发送（远端 DNS）。
```

- [ ] **Step 2: 全套验证**

Run（工作区根目录）：
```bash
go build ./...
go vet ./...
go test ./... -count=1
```
Run（relaykit 独立模块）：
```bash
cd relaykit; $env:GOWORK='off'; go build ./...; go test ./...
```
Run（web）：
```bash
bun run typecheck
bunx vitest run
```
Expected: 全部通过；service 包无 flaky 失败（Task 3 集成测试使用随机端口回环，无时间戳碰撞）

- [ ] **Step 3: 提交**

```bash
git add docs/spec/outbound-proxy-module.md
git commit -m "docs(spec): document shadowsocks proxy protocol support"
```

---

## Self-Review 记录

- **Spec 覆盖**：
  - URL 变体/归一化 → Task 1；协议白名单/批量导入 → Task 2；传输层 → Task 3+4；前端 → Task 5；文档 → Task 6。全部覆盖
  - 「密码含保留字符提示 base64」→ Task 1 测试 `password with hash treated as fragment` 覆盖行为；错误信息由 Task 2 sanitize 透传
  - 「零模型迁移」→ 无表结构改动（Task 2 仅常量/白名单）
  - 「relaykit 不受影响」→ Task 6 Step 2 验证
- **占位符扫描**：无 TBD/TODO；每个 step 含完整代码或命令
- **类型一致性**：`newShadowsocksDialer(method, password string, host string, port int, forwardDialer *net.Dialer)` 在 Task 3 定义、Task 4 调用，签名一致；`shadowsocksConnect(conn net.Conn, target socks.Addr)` 仅 Task 3 内部；`socks.Error(reply[1])` 使用 v0.1.5 已确认的 `type Error byte`
- **已知偏差（已修正 spec）**：AEAD 白名单按 v0.1.5 实际支持 3 种（spec 原写 5 种，已更新）