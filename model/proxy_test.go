package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProxyEndpointRequiresExplicitProtocolAndHTTPPort(t *testing.T) {
	protocol, host, port, username, password, err := ParseProxyEndpoint("socks5h://user:password@[2001:db8::1]")
	require.NoError(t, err)
	assert.Equal(t, ProxyProtocolSOCKS5H, protocol)
	assert.Equal(t, "2001:db8::1", host)
	assert.Equal(t, 1080, port)
	assert.Equal(t, "user", username)
	assert.Equal(t, "password", password)

	_, _, _, _, _, err = ParseProxyEndpoint("example.com:8080")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxy URL")

	_, _, _, _, _, err = ParseProxyEndpoint("https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestProxySummaryDoesNotExposeCredentials(t *testing.T) {
	proxy := &Proxy{ID: 1, Name: "test", Protocol: ProxyProtocolHTTP, Host: "example.com", Port: 8080, Status: ProxyStatusActive, UsernameEncrypted: "ciphertext"}
	summary := proxy.Summary()
	assert.True(t, summary.CredentialConfigured)
	assert.Empty(t, summary.CredentialDecryptFailed)
	assert.Equal(t, "example.com", summary.Host)
}

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
