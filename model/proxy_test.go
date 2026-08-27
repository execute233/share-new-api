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
