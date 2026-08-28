package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCodexDisguiseRoutesRegisterWithoutConflict(t *testing.T) {
	setupRelayRouterTestDB(t)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	tests := []struct {
		name string
		path string
	}{
		{name: "responses", path: "/backend-api/codex/responses"},
		{name: "responses compact", path: "/backend-api/codex/responses/compact"},
		{name: "alpha search", path: "/backend-api/codex/alpha/search"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			engine.ServeHTTP(recorder, request)
			// 未带 token 应返回 401（而非 404），证明路由已挂载
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}