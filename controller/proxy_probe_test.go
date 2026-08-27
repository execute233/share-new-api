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
