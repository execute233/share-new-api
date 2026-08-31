package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCallAuditReturnsMatchingDraftRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body, err := common.Marshal(ToolCallAuditTestRequest{
		Settings: setting.ToolCallAuditSettings{
			Version:               1,
			Mode:                  setting.ToolCallAuditModeDisabled,
			MaxArgumentBytes:      1024,
			MaxTotalArgumentBytes: 2048,
			MaxJSONDepth:          8,
			LogRetentionDays:      7,
			Rules: []setting.ToolCallAuditRule{{
				ID:        "private-key",
				Name:      "Private key",
				Enabled:   true,
				MatchType: "contains",
				Patterns:  []string{"id_rsa"},
			}},
		},
		ToolName:  "shell",
		Arguments: `{"cmd":"cat ~/.ssh/id_rsa"}`,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/option/tool-call-audit/test", bytes.NewReader(body))
	TestToolCallAudit(context)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Matched bool `json:"matched"`
			Blocked bool `json:"blocked"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.True(t, response.Data.Matched)
	assert.True(t, response.Data.Blocked)
}

func TestGetToolCallAuditDefaultsReturnsCanonicalRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/option/tool-call-audit/defaults", nil)

	GetToolCallAuditDefaults(context)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Rules []setting.ToolCallAuditRule `json:"rules"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.NotEmpty(t, response.Data.Rules)
	var environmentRule *setting.ToolCallAuditRule
	for i := range response.Data.Rules {
		if response.Data.Rules[i].ID == "credential-access-env" {
			environmentRule = &response.Data.Rules[i]
			break
		}
	}
	require.NotNil(t, environmentRule)
	assert.Contains(t, environmentRule.Patterns, "git-credentials")
}
