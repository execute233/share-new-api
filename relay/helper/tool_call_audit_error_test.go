package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCallBlockedStreamErrorUsesClientProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		relayFormat types.RelayFormat
		wantEvent   string
	}{
		{name: "openai chat", relayFormat: types.RelayFormatOpenAI},
		{name: "responses", relayFormat: types.RelayFormatOpenAIResponses, wantEvent: "event: error"},
		{name: "claude", relayFormat: types.RelayFormatClaude, wantEvent: "event: error"},
		{name: "gemini", relayFormat: types.RelayFormatGemini},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/v1/stream", strings.NewReader(""))
			context.Set(common.RequestIdKey, "req-audit")

			require.NoError(t, ToolCallBlockedStreamError(context, test.relayFormat))

			body := recorder.Body.String()
			assert.Contains(t, body, string(types.ErrorCodeToolCallBlocked))
			assert.Contains(t, body, "req-audit")
			if test.wantEvent != "" {
				assert.Contains(t, body, test.wantEvent)
			}
		})
	}
}
