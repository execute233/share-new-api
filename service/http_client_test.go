package service

import (
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
