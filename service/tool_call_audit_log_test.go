package service

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamArgumentLimitLogPreservesOriginalEvidence(t *testing.T) {
	db := setupToolCallAuditLogTestDB(t)
	config := setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeAudit,
		MaxArgumentBytes:      16,
		MaxTotalArgumentBytes: 16,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
	}
	setToolCallAuditTestSettings(t, config)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Set(common.RequestIdKey, "req-limit")
	context.Set("id", 88)

	arguments := `{"cmd":"abcdefghijklmnop"}`
	gate := NewToolCallAuditStreamGate(context, 0, "model", ToolCallAuditStreamOpenAIChat)
	event := fmt.Sprintf(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"shell","arguments":%q}}]}}]}`, arguments)
	require.True(t, gate.Observe(event))
	pending, auditErr := gate.Finish()
	assert.Nil(t, pending)
	require.NotNil(t, auditErr)

	var stored model.Log
	require.NoError(t, db.First(&stored).Error)
	var other struct {
		AdminInfo struct {
			ToolCallAudit struct {
				Arguments     string `json:"arguments"`
				ArgumentsHash string `json:"arguments_sha256"`
				ArgumentsSize int    `json:"arguments_size"`
			} `json:"tool_call_audit"`
		} `json:"admin_info"`
	}
	require.NoError(t, common.UnmarshalJsonStr(stored.Other, &other))
	wantHash := fmt.Sprintf("%x", sha256.Sum256([]byte(arguments)))
	assert.Equal(t, arguments, other.AdminInfo.ToolCallAudit.Arguments)
	assert.Equal(t, wantHash, other.AdminInfo.ToolCallAudit.ArgumentsHash)
	assert.Equal(t, len(arguments), other.AdminInfo.ToolCallAudit.ArgumentsSize)
}

func TestRecordToolCallAuditBlockStoresAdminEvidenceOnly(t *testing.T) {
	db := setupToolCallAuditLogTestDB(t)
	config := setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{{
			ID:        "private-key",
			Enabled:   true,
			MatchType: "contains",
			Patterns:  []string{"id_rsa"},
		}},
	}
	arguments := `{"path":"~/.ssh/id_rsa"}`
	call := NormalizedToolCallFromString("openai", "call-1", "shell", arguments)
	result := EvaluateToolCallAuditWithSettings(config, 1, call, len(call.RawArguments))
	require.True(t, result.Blocked)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Set(common.RequestIdKey, "req-audit")
	context.Set("id", 77)
	RecordToolCallAuditBlock(context, 0, "model", config.Version, result, call)

	var stored model.Log
	require.NoError(t, db.First(&stored).Error)
	assert.Equal(t, model.LogTypeToolCallAudit, stored.Type)
	assert.Equal(t, 0, stored.UserId)
	assert.Equal(t, "req-audit", stored.RequestId)
	var other struct {
		AdminInfo struct {
			ToolCallAudit struct {
				Arguments     string `json:"arguments"`
				ArgumentsHash string `json:"arguments_sha256"`
				ArgumentsSize int    `json:"arguments_size"`
				UserID        int    `json:"user_id"`
			} `json:"tool_call_audit"`
		} `json:"admin_info"`
	}
	require.NoError(t, common.UnmarshalJsonStr(stored.Other, &other))
	assert.Equal(t, arguments, other.AdminInfo.ToolCallAudit.Arguments)
	assert.Equal(t, result.ArgumentsHash, other.AdminInfo.ToolCallAudit.ArgumentsHash)
	assert.Equal(t, len(arguments), other.AdminInfo.ToolCallAudit.ArgumentsSize)
	assert.Equal(t, 77, other.AdminInfo.ToolCallAudit.UserID)

	userLogs, total, err := model.GetUserLogs(77, model.LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, userLogs)
}
