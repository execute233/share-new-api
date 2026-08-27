package common

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// supportedShadowsocksMethods 是 go-shadowsocks2 v0.1.5 支持的 AEAD 加密方法，
// 键大小写不敏感（兼容 SIP002 常用大写别名），值为规范小写名。
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

// ParseProxyURLStrict validates and normalizes a proxy URL for persistence.
func ParseProxyURLStrict(rawProxyURL string) (*url.URL, error) {
	parsedURL, _, err := parseProxyURL(rawProxyURL, false)
	return parsedURL, err
}

// ParseProxyURLRuntime validates and normalizes a proxy URL for runtime use.
// The boolean result reports whether a legacy path, query, or fragment was removed.
func ParseProxyURLRuntime(rawProxyURL string) (*url.URL, bool, error) {
	return parseProxyURL(rawProxyURL, true)
}

func parseProxyURL(rawProxyURL string, allowLegacySuffix bool) (*url.URL, bool, error) {
	trimmedProxyURL := strings.TrimSpace(rawProxyURL)
	if trimmedProxyURL == "" {
		return nil, false, nil
	}

	parsedURL, err := url.Parse(trimmedProxyURL)
	if err != nil {
		return nil, false, fmt.Errorf("invalid proxy URL")
	}
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	switch parsedURL.Scheme {
	case "http", "https", "socks5", "socks5h", "ss":
	default:
		return nil, false, fmt.Errorf("proxy URL must use http, https, socks5, socks5h, or ss")
	}
	if parsedURL.Scheme == "ss" {
		if err := normalizeShadowsocksUserInfo(parsedURL); err != nil {
			return nil, false, err
		}
	}
	if parsedURL.Hostname() == "" {
		return nil, false, fmt.Errorf("proxy URL must include a host")
	}
	if portText := parsedURL.Port(); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, false, fmt.Errorf("proxy URL must include a valid port")
		}
	}

	hasQuery := parsedURL.RawQuery != "" || parsedURL.ForceQuery
	hasFragment := strings.Contains(trimmedProxyURL, "#")
	escapedPath := parsedURL.EscapedPath()
	hasNonRootPath := escapedPath != "" && escapedPath != "/"
	legacySuffixStripped := hasQuery || hasFragment || hasNonRootPath
	if !allowLegacySuffix {
		switch {
		case hasQuery:
			return nil, false, fmt.Errorf("proxy URL must not include a query")
		case hasFragment:
			return nil, false, fmt.Errorf("proxy URL must not include a fragment")
		case hasNonRootPath:
			return nil, false, fmt.Errorf("proxy URL must not include a path")
		}
	}

	parsedURL.Path = ""
	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.ForceQuery = false
	parsedURL.Fragment = ""
	parsedURL.RawFragment = ""

	if (parsedURL.Scheme == "socks5" || parsedURL.Scheme == "socks5h") && parsedURL.Port() == "" {
		parsedURL.Host = net.JoinHostPort(parsedURL.Hostname(), "1080")
	}
	if parsedURL.Scheme == "ss" && parsedURL.Port() == "" {
		parsedURL.Host = net.JoinHostPort(parsedURL.Hostname(), "8388")
	}

	return parsedURL, legacySuffixStripped, nil
}
