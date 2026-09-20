package channel_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type websocketProxyTestAdaptor struct {
	channel.Adaptor
	url string
}

func (a websocketProxyTestAdaptor) GetRequestURL(*relaycommon.RelayInfo) (string, error) {
	return a.url, nil
}

func (a websocketProxyTestAdaptor) SetupRequestHeader(_ *gin.Context, _ *http.Header, _ *relaycommon.RelayInfo) error {
	return nil
}

func TestWebSocketUsesShadowsocksTransport(t *testing.T) {
	// The in-process test client and server share the library's replay filter.
	t.Setenv("SHADOWSOCKS_SF_CAPACITY", "0")
	service.InitHttpClient()
	t.Cleanup(service.ResetProxyClientCache)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()
		assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("proxied websocket")))
	}))
	defer upstream.Close()
	cipher, err := core.PickCipher("chacha20-ietf-poly1305", nil, "test-secret")
	require.NoError(t, err)
	listener, err := core.Listen("tcp", "127.0.0.1:0", cipher)
	require.NoError(t, err)
	defer listener.Close()
	targets := make(chan string, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		target, err := socks.ReadAddr(conn)
		if err != nil {
			return
		}
		targets <- target.String()
		remote, err := net.DialTimeout("tcp", strings.TrimPrefix(upstream.URL, "http://"), 5*time.Second)
		if err != nil {
			return
		}
		defer remote.Close()
		go func() { _, _ = io.Copy(remote, conn) }()
		_, _ = io.Copy(conn, remote)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil).WithContext(ctx)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelSetting: dto.ChannelSettings{Proxy: "ss://chacha20-ietf-poly1305:test-secret@" + listener.Addr().String()},
	}}
	// This hostname cannot connect directly. The proxy deliberately routes it to
	// the local upstream, proving the WebSocket handshake uses the selected exit.
	conn, err := channel.DoWssRequest(websocketProxyTestAdaptor{url: "ws://upstream.invalid/v1/responses"}, c, info, nil)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "proxied websocket", string(message))
	assert.Equal(t, "upstream.invalid:80", <-targets)
	<-finished
}
