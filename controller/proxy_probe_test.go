package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedProxyProbe struct{ result service.ProxyProbeResult }

func (p fixedProxyProbe) Probe(context.Context, service.ResolvedProxy, bool) service.ProxyProbeResult {
	return p.result
}

func TestProxyQualityCheckPersistsLatestWithoutChangingStatus(t *testing.T) {
	setupProxyControllerTestDB(t)
	proxy := model.Proxy{Name: "quality", Protocol: model.ProxyProtocolHTTP, Host: "example.com", Port: 8080, Status: model.ProxyStatusInactive}
	require.NoError(t, proxy.Insert())
	previous := proxyProbe
	proxyProbe = fixedProxyProbe{result: service.ProxyProbeResult{
		LatencyMS:     123,
		HTTPStatus:    200,
		IPAddress:     "203.0.113.8",
		Country:       "Testland",
		QualityStatus: "healthy",
		QualityScore:  100,
		QualityGrade:  "A",
		Summary:       "ok",
	}}
	t.Cleanup(func() { proxyProbe = previous })

	router := gin.New()
	router.POST("/proxy/:id/quality-check", CheckProxyQuality)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/proxy/%d/quality-check", proxy.ID), nil))

	assert.Contains(t, recorder.Body.String(), `"quality_grade":"A"`)
	assert.NotContains(t, strings.ToLower(recorder.Body.String()), "password")
	stored, err := model.GetProxyByID(proxy.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ProxyStatusInactive, stored.Status)
	assert.Equal(t, "203.0.113.8", stored.IPAddress)
	assert.Equal(t, 100, *stored.QualityScore)
}

func TestProxyQualityCheckReturnsPerTargetItems(t *testing.T) {
	setupProxyControllerTestDB(t)
	proxy := model.Proxy{Name: "quality-items", Protocol: model.ProxyProtocolHTTP, Host: "example.com", Port: 8080, Status: model.ProxyStatusInactive}
	require.NoError(t, proxy.Insert())
	previous := proxyProbe
	proxyProbe = fixedProxyProbe{result: service.ProxyProbeResult{
		LatencyMS:     123,
		HTTPStatus:    200,
		IPAddress:     "203.0.113.8",
		Country:       "Testland",
		QualityStatus: "warn",
		QualityScore:  70,
		QualityGrade:  "B",
		Summary:       "通过 1 项，告警 1 项，失败 2 项，挑战 1 项",
		Items: []model.ProxyQualityItem{
			{Target: "base_connectivity", Status: "pass", HTTPStatus: 200, LatencyMS: 123, Message: "代理出口连通正常"},
			{Target: "openai", URL: "https://api.openai.com/v1/models", Status: "pass", HTTPStatus: 401, LatencyMS: 300, Message: "目标可达"},
			{Target: "anthropic", URL: "https://api.anthropic.com/v1/messages", Status: "challenge", HTTPStatus: 403, LatencyMS: 500, Message: "目标返回 Cloudflare 挑战", CFRay: "abc123"},
			{Target: "gemini", URL: "https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta", Status: "fail", LatencyMS: 1000, Message: "探测请求失败: context deadline exceeded"},
			{Target: "grok", URL: "https://api.x.ai/v1/models", Status: "warn", HTTPStatus: 429, LatencyMS: 800, Message: "目标被限流"},
		},
	}}
	t.Cleanup(func() { proxyProbe = previous })

	router := gin.New()
	router.POST("/proxy/:id/quality-check", CheckProxyQuality)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/proxy/%d/quality-check", proxy.ID), nil))

	body := recorder.Body.String()
	assert.Contains(t, body, `"quality_items":[`)
	assert.Contains(t, body, `"target":"base_connectivity"`)
	assert.Contains(t, body, `"url":"https://api.openai.com/v1/models"`)
	assert.Contains(t, body, `"status":"challenge"`)
	assert.Contains(t, body, `"cf_ray":"abc123"`)
	assert.Contains(t, body, `"http_status":429`)
	stored, err := model.GetProxyByID(proxy.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ProxyStatusInactive, stored.Status)
	assert.Equal(t, 70, *stored.QualityScore)
}

func TestProxyTestResponseOmitsQualityItems(t *testing.T) {
	setupProxyControllerTestDB(t)
	proxy := model.Proxy{Name: "test-omit", Protocol: model.ProxyProtocolHTTP, Host: "example.com", Port: 8080}
	require.NoError(t, proxy.Insert())
	previous := proxyProbe
	proxyProbe = fixedProxyProbe{result: service.ProxyProbeResult{
		LatencyMS:     50,
		HTTPStatus:    200,
		IPAddress:     "203.0.113.9",
		QualityStatus: "healthy",
		QualityScore:  100,
		QualityGrade:  "A",
		Summary:       "ok",
		Items: []model.ProxyQualityItem{
			{Target: "base_connectivity", Status: "pass", HTTPStatus: 200, LatencyMS: 50, Message: "代理出口连通正常"},
		},
	}}
	t.Cleanup(func() { proxyProbe = previous })

	router := gin.New()
	router.POST("/proxy/:id/test", TestProxy)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/proxy/%d/test", proxy.ID), nil))

	assert.NotContains(t, recorder.Body.String(), "quality_items")
}
