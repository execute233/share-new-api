package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setToolCallAuditTestSettings(t *testing.T, config setting.ToolCallAuditSettings) {
	t.Helper()
	previous := setting.GetToolCallAuditSettings()
	t.Cleanup(func() {
		data, err := common.Marshal(previous)
		require.NoError(t, err)
		require.NoError(t, setting.UpdateToolCallAuditSettings(string(data)))
	})
	data, err := common.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, setting.UpdateToolCallAuditSettings(string(data)))
}

func TestEvaluateToolCallAuditChecksRawAndStructuredArguments(t *testing.T) {
	config := setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{
			{ID: "path", Enabled: true, MatchType: "contains", Patterns: []string{".ssh"}},
		},
	}

	structured := NewNormalizedToolCall("openai", "call-1", "shell", []byte(`{"path":"/home/user/.ssh/id_rsa"}`))
	result := EvaluateToolCallAuditWithSettings(config, 1, structured, len(structured.RawArguments))
	require.True(t, result.Blocked)
	assert.Equal(t, "path", result.Matches[0].RuleID)

	malformed := NewNormalizedToolCall("openai", "call-2", "shell", []byte(`{"path":"/home/user/.ssh`))
	assert.False(t, malformed.Parsed)
	result = EvaluateToolCallAuditWithSettings(config, 1, malformed, len(malformed.RawArguments))
	assert.True(t, result.Blocked)
}

func TestAuditToolCallsAuditModeDoesNotBlock(t *testing.T) {
	config := setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeAudit,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{
			{ID: "danger", Enabled: true, MatchType: "contains", Patterns: []string{"danger"}},
		},
	}

	call := NormalizedToolCallFromString("openai", "call-1", "shell", `{"cmd":"danger"}`)
	result := EvaluateToolCallAuditWithSettings(config, 1, call, len(call.RawArguments))
	assert.False(t, result.Blocked)
	assert.NotEmpty(t, result.Matches)
}

func TestAuditToolCallsDisabledChannelDoesNotBlock(t *testing.T) {
	config := setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		ChannelIDs:            []int{2},
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{
			{ID: "danger", Enabled: true, MatchType: "contains", Patterns: []string{"danger"}},
		},
	}

	call := NormalizedToolCallFromString("openai", "call-1", "shell", `{"cmd":"danger"}`)
	result := EvaluateToolCallAuditWithSettings(config, 1, call, len(call.RawArguments))
	assert.False(t, result.Enabled)
	assert.False(t, result.Blocked)
}

func TestPreAuditUpstreamStreamBlocksCompleteToolCallsAcrossProtocols(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      4096,
		MaxTotalArgumentBytes: 8192,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{
			{ID: "private-key", Enabled: true, MatchType: "contains", Patterns: []string{"id_rsa"}},
		},
	})

	tests := []struct {
		name string
		body string
	}{
		{
			name: "openai chat",
			body: "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"shell\",\"arguments\":\"{\\\"path\\\":\\\"\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"~/.ssh/id_rsa\\\"}\"}}]}}]}\n\n",
		},
		{
			name: "responses",
			body: "data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"call_id\":\"c1\",\"name\":\"shell\"}}\n\n" +
				"data: {\"type\":\"response.function_call_arguments.delta\",\"output_index\":0,\"delta\":\"{\\\"path\\\":\\\"~/.ssh/id_rsa\\\"}\"}\n\n",
		},
		{
			name: "claude",
			body: "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"c1\",\"name\":\"shell\",\"input\":{}}}\n\n" +
				"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"~/.ssh/id_rsa\\\"}\"}}\n\n",
		},
		{
			name: "gemini",
			body: "data: {\"candidates\":[{\"index\":0,\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"shell\",\"args\":{\"path\":\"~/.ssh/id_rsa\"}}}]}}]}\n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &http.Response{Body: io.NopCloser(strings.NewReader(test.body))}
			err := PreAuditUpstreamStream(nil, 1, "model", response)
			require.NotNil(t, err)
			assert.Equal(t, types.ErrorCodeToolCallBlocked, err.GetErrorCode())
		})
	}
}

func TestPreAuditUpstreamStreamRestoresAllowedBody(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      4096,
		MaxTotalArgumentBytes: 8192,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{
			{ID: "private-key", Enabled: true, MatchType: "contains", Patterns: []string{"id_rsa"}},
		},
	})
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n"
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}

	require.Nil(t, PreAuditUpstreamStream(nil, 1, "model", response))
	restored, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, body, string(restored))
}

func TestEvaluateToolCallAuditSupportsConfiguredMatchersAndPathScope(t *testing.T) {
	previousWords := append([]string(nil), setting.SensitiveWords...)
	t.Cleanup(func() { setting.SensitiveWords = previousWords })
	setting.SensitiveWords = []string{"blocked-keyword"}

	tests := []struct {
		name      string
		matchType string
		patterns  []string
		paths     []string
		arguments string
		want      bool
	}{
		{name: "contains", matchType: "contains", patterns: []string{"id_rsa"}, arguments: `{"cmd":"cat id_rsa"}`, want: true},
		{name: "exact selected path", matchType: "exact", patterns: []string{"safe"}, paths: []string{"cmd"}, arguments: `{"cmd":"safe","path":"danger"}`, want: true},
		{name: "glob", matchType: "glob", patterns: []string{"*.env"}, paths: []string{"path"}, arguments: `{"path":"prod.env"}`, want: true},
		{name: "regex", matchType: "regex", patterns: []string{`(?i)secret_[a-z]+`}, arguments: `{"token":"SECRET_VALUE"}`, want: true},
		{name: "keyword set", matchType: "keyword_set", patterns: []string{"SensitiveWords"}, arguments: `{"cmd":"blocked-keyword"}`, want: true},
		{name: "path excludes other leaf", matchType: "contains", patterns: []string{"danger"}, paths: []string{"cmd"}, arguments: `{"cmd":"safe","path":"danger"}`, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := setting.ToolCallAuditSettings{
				Version:               1,
				Mode:                  setting.ToolCallAuditModeBlock,
				MaxArgumentBytes:      4096,
				MaxTotalArgumentBytes: 8192,
				MaxJSONDepth:          8,
				LogRetentionDays:      7,
				Rules: []setting.ToolCallAuditRule{{
					ID:            "rule",
					Enabled:       true,
					MatchType:     test.matchType,
					Patterns:      test.patterns,
					ArgumentPaths: test.paths,
				}},
			}
			call := NormalizedToolCallFromString("test", "call", "shell", test.arguments)
			result := EvaluateToolCallAuditWithSettings(config, 1, call, len(call.RawArguments))
			assert.Equal(t, test.want, result.Blocked)
		})
	}
}
