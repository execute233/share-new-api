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
			name:    "password with hash rejected",
			raw:     "ss://aes-256-gcm:pa#ss@proxy.example.com:8388",
			wantErr: "invalid proxy URL",
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
