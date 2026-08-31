package claude

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setClaudeToolAuditSettings(t *testing.T) {
	t.Helper()
	previous := setting.GetToolCallAuditSettings()
	config := setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      4096,
		MaxTotalArgumentBytes: 8192,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{{
			ID:            "private-key",
			Enabled:       true,
			ToolNames:     []string{"shell"},
			ArgumentPaths: []string{"path"},
			MatchType:     "contains",
			Patterns:      []string{"id_rsa"},
		}},
	}
	data, err := common.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, setting.UpdateToolCallAuditSettings(string(data)))
	t.Cleanup(func() {
		previousData, marshalErr := common.Marshal(previous)
		require.NoError(t, marshalErr)
		require.NoError(t, setting.UpdateToolCallAuditSettings(string(previousData)))
	})
}

func TestHandleClaudeResponseDataBlocksToolBeforeWritingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setClaudeToolAuditSettings(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 1, UpstreamModelName: "claude-test"},
		RelayFormat: types.RelayFormatClaude,
	}
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}
	data := []byte(`{"type":"message","content":[{"type":"tool_use","id":"tu1","name":"shell","input":{"path":"~/.ssh/id_rsa"}}]}`)

	auditErr := HandleClaudeResponseData(c, info, claudeInfo, nil, data)

	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
	assert.Empty(t, recorder.Body.String())
}

func TestHandleClaudeResponseDataCountsToolUse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.SetToolPriceForTest("lookup_fn", 3.0)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("lookup_fn")
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet",
		RelayFormat:     types.RelayFormatClaude,
	}
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}

	data := []byte(`{
		"type":"message",
		"content":[
			{"type":"text","text":"hi"},
			{"type":"tool_use","id":"tu1","name":"lookup_fn","input":{}},
			{"type":"server_tool_use","id":"stu1","name":"web_search","input":{}}
		],
		"usage":{"input_tokens":1,"output_tokens":1}
	}`)

	err := HandleClaudeResponseData(c, info, claudeInfo, nil, data)
	require.Nil(t, err)
	require.NotNil(t, info.ResponsesUsageInfo)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "lookup_fn")
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["lookup_fn"].CallCount)
	assert.NotContains(t, info.ResponsesUsageInfo.BuiltInTools, "web_search")
}

func TestCountClaudeStreamBillableToolsSetsWebSearchRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := &relaycommon.RelayInfo{OriginModelName: "claude-3-7-sonnet"}

	countClaudeStreamBillableTools(c, info, &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			ServerToolUse: &dto.ClaudeServerToolUse{WebSearchRequests: 3},
		},
	})
	assert.Equal(t, 3, c.GetInt("claude_web_search_requests"))

	operation_setting.SetToolPriceForTest("stream_fn", 2.0)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("stream_fn")
	})
	countClaudeStreamBillableTools(c, info, &dto.ClaudeResponse{
		Type: "content_block_start",
		ContentBlock: &dto.ClaudeMediaMessage{
			Type: "tool_use",
			Name: "stream_fn",
		},
	})
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "stream_fn")
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["stream_fn"].CallCount)
}
