package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试客户端与服务端同进程运行，go-shadowsocks2 的进程级 salt 重放防护
// （internal.BloomRing）会把客户端登记的 salt 误判为重复，禁用之。
func init() {
	_ = os.Setenv("SHADOWSOCKS_SF_CAPACITY", "0")
}

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
				target, err := socks.ReadAddr(conn)
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
