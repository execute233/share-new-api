package controller

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProxyPoolRoutesManagementAndRelayContext(t *testing.T) {
	db := setupProxyControllerTestDB(t)
	service.InitHttpClient()
	t.Cleanup(service.ResetProxyClientCache)
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "upstream.invalid", r.Host)
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"proxied-model"}]}`))
		case "/metrics":
			w.WriteHeader(http.StatusNoContent)
		case "/version":
			_, _ = w.Write([]byte(`{"version":"proxy-version"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer proxyServer.Close()
	endpoint, err := url.Parse(proxyServer.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(endpoint.Port())
	require.NoError(t, err)
	proxy := model.Proxy{Name: "integration", Protocol: model.ProxyProtocolHTTP, Host: endpoint.Hostname(), Port: port, Status: model.ProxyStatusActive}
	require.NoError(t, db.Create(&proxy).Error)
	baseURL := "http://upstream.invalid"
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Name: "test", Key: "key", BaseURL: &baseURL, ProxyID: &proxy.ID}
	channel.SetSetting(dto.ChannelSettings{Proxy: "http://legacy.invalid:1"})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, channel, "model"))
	settings, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	require.True(t, ok)
	assert.Equal(t, proxyServer.URL, settings.Proxy)
	assert.Equal(t, "http://legacy.invalid:1", channel.GetSetting().Proxy, "runtime resolution must not mutate persisted settings")
	models, err := fetchChannelUpstreamModelIDs(channel)
	require.NoError(t, err)
	assert.Equal(t, []string{"proxied-model"}, models)
	body, err := GetResponseBody(http.MethodGet, baseURL+"/balance", channel, http.Header{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(body))
	channel.Type = constant.ChannelTypeVLLM
	status, err := fetchInferenceStatus(context.Background(), channel)
	require.NoError(t, err)
	assert.Equal(t, "proxy-version", status.Version)
}

func TestProxyPoolManagementHonorsChannelTLS(t *testing.T) {
	db := setupProxyControllerTestDB(t)
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1beta/models" {
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-test"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"proxied-model"}]}`))
	}))
	t.Cleanup(upstream.Close)
	var connections atomic.Int64
	connectProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		target, err := net.Dial("tcp", r.Host)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer target.Close()
		conn, buffered, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		connections.Add(1)
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() { _, _ = io.Copy(target, buffered) }()
		_, _ = io.Copy(conn, target)
	}))
	t.Cleanup(connectProxy.Close)
	service.InitHttpClient()
	t.Cleanup(service.ResetProxyClientCache)
	endpoint, err := url.Parse(connectProxy.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(endpoint.Port())
	require.NoError(t, err)
	proxy := model.Proxy{Name: "tls-integration", Protocol: model.ProxyProtocolHTTP, Host: endpoint.Hostname(), Port: port, Status: model.ProxyStatusActive}
	require.NoError(t, db.Create(&proxy).Error)
	for _, tc := range []struct {
		name        string
		channelType int
		skipVerify  bool
		wantModels  []string
	}{
		{name: "OpenAI verifies", channelType: constant.ChannelTypeOpenAI},
		{name: "OpenAI skips verification", channelType: constant.ChannelTypeOpenAI, skipVerify: true, wantModels: []string{"proxied-model"}},
		{name: "Gemini verifies", channelType: constant.ChannelTypeGemini},
		{name: "Gemini skips verification", channelType: constant.ChannelTypeGemini, skipVerify: true, wantModels: []string{"gemini-test"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := &model.Channel{Type: tc.channelType, Key: "key", BaseURL: &upstream.URL, ProxyID: &proxy.ID}
			channel.SetSetting(dto.ChannelSettings{Proxy: "http://legacy.invalid:1", TLSInsecureSkipVerify: tc.skipVerify})
			models, err := fetchChannelUpstreamModelIDs(channel)
			if tc.skipVerify {
				require.NoError(t, err)
				assert.Equal(t, tc.wantModels, models)
				body, err := GetResponseBody(http.MethodGet, upstream.URL+"/balance", channel, http.Header{})
				require.NoError(t, err)
				assert.JSONEq(t, `{"data":[{"id":"proxied-model"}]}`, string(body))
			} else {
				require.ErrorContains(t, err, "certificate")
				_, err = GetResponseBody(http.MethodGet, upstream.URL+"/balance", channel, http.Header{})
				require.ErrorContains(t, err, "certificate")
			}
			assert.Equal(t, "http://legacy.invalid:1", channel.GetSetting().Proxy)
		})
	}
	assert.Positive(t, connections.Load(), "management requests must pass through the bound proxy")
}

func TestChannelProxyBindingClearIsAtomic(t *testing.T) {
	db := setupProxyControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	id := 12
	channel := model.Channel{Name: "original", Key: "key", Type: constant.ChannelTypeOpenAI, Models: "model", Group: "default", ProxyID: &id}
	require.NoError(t, db.Create(&channel).Error)
	channel.Name = "updated"
	channel.ProxyID = nil
	require.NoError(t, channel.UpdateWithProxyBinding())
	var saved model.Channel
	require.NoError(t, db.First(&saved, channel.Id).Error)
	assert.Nil(t, saved.ProxyID)
	assert.Equal(t, "updated", saved.Name)
	saved.ProxyID = &id
	require.NoError(t, saved.UpdateWithProxyBinding())
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fail_proxy_update", func(tx *gorm.DB) {
		if values, ok := tx.Statement.Dest.(map[string]any); ok {
			if _, hasProxy := values["proxy_id"]; hasProxy {
				tx.AddError(errors.New("proxy write failed"))
			}
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("fail_proxy_update") })
	saved.Name = "must roll back"
	saved.ProxyID = nil
	require.ErrorContains(t, saved.UpdateWithProxyBinding(), "proxy write failed")
	var unchanged model.Channel
	require.NoError(t, db.First(&unchanged, channel.Id).Error)
	assert.Equal(t, "updated", unchanged.Name)
	require.NotNil(t, unchanged.ProxyID)
	assert.Equal(t, id, *unchanged.ProxyID)
}

func TestModelPreviewProxyBindingPresence(t *testing.T) {
	for _, tc := range []struct {
		body    string
		present bool
		id      *int
	}{
		{`{}`, false, nil}, {`{"proxy_id":null}`, true, nil}, {`{"proxy_id":3}`, true, common.GetPointer(3)},
	} {
		var req fetchModelsRequest
		require.NoError(t, common.Unmarshal([]byte(tc.body), &req))
		assert.Equal(t, tc.present, req.ProxyIDSet)
		assert.Equal(t, tc.id, req.ProxyID)
	}
}
