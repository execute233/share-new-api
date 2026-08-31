package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditResponsesResponseUnwrapsStringArgumentsForPathRules(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{{
			ID:            "private-key",
			Enabled:       true,
			ToolNames:     []string{"shell"},
			ArgumentPaths: []string{"cmd"},
			MatchType:     "contains",
			Patterns:      []string{"id_rsa"},
		}},
	})

	response := &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{{
		Type:      dto.BuildInCallFunctionCall,
		CallId:    "call-1",
		Name:      "shell",
		Arguments: json.RawMessage(`"{\"cmd\":\"cat ~/.ssh/id_rsa\"}"`),
	}}}

	auditErr := AuditResponsesResponse(nil, 1, "model", response)
	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestAuditToolCallsBlocksJSONBeyondDepthLimit(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          2,
		LogRetentionDays:      7,
	})

	call := NormalizedToolCallFromString("openai", "call-1", "shell", `{"a":{"b":{"c":"value"}}}`)
	auditErr := AuditToolCalls(nil, 1, "model", []NormalizedToolCall{call})

	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestAuditToolCallsAllowsJSONAtDepthLimit(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          4,
		LogRetentionDays:      7,
	})
	call := NormalizedToolCallFromString("openai", "call-1", "shell", `{"a":{"b":{"c":"safe"}}}`)

	auditErr := AuditToolCalls(nil, 1, "model", []NormalizedToolCall{call})

	assert.Nil(t, auditErr)
}

func TestAuditToolCallsCountsRepeatedLogicalCallsTowardTotalLimit(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      32,
		MaxTotalArgumentBytes: 48,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
	})

	call := NormalizedToolCallFromString("openai", "same-id", "shell", `{"cmd":"abcdefghijklmnop"}`)
	require.LessOrEqual(t, len(call.RawArguments), 32)
	require.Greater(t, len(call.RawArguments)*2, 48)
	auditErr := AuditToolCalls(nil, 1, "model", []NormalizedToolCall{call, call})

	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestAuditToolCallsAppliesDotPathToArrayElements(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{{
			ID:            "private-key",
			Enabled:       true,
			ArgumentPaths: []string{"items.cmd"},
			MatchType:     "contains",
			Patterns:      []string{"id_rsa"},
		}},
	})

	call := NormalizedToolCallFromString("openai", "call-1", "batch", `{"items":[{"cmd":"cat ~/.ssh/id_rsa"}]}`)
	auditErr := AuditToolCalls(nil, 1, "model", []NormalizedToolCall{call})

	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestAuditToolCallsTraversesNestedWildcardPaths(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []setting.ToolCallAuditRule{{
			ID:            "private-key",
			Enabled:       true,
			ArgumentPaths: []string{"targets.*.cmd"},
			MatchType:     "contains",
			Patterns:      []string{"id_rsa"},
		}},
	})
	call := NormalizedToolCallFromString("openai", "call-1", "batch", `{"targets":{"primary":{"cmd":"cat ~/.ssh/id_rsa"}}}`)

	auditErr := AuditToolCalls(nil, 1, "model", []NormalizedToolCall{call})

	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestToolCallAuditStreamGatePassesTextAndBlocksBufferedToolEvents(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
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
	})

	gate := NewToolCallAuditStreamGate(nil, 1, "model", ToolCallAuditStreamOpenAIChat)
	assert.False(t, gate.Observe(`{"choices":[{"index":0,"delta":{"content":"hello"}}]}`))
	assert.True(t, gate.Observe(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"shell","arguments":"{\"path\":\""}}]}}]}`))
	assert.True(t, gate.Observe(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"~/.ssh/id_rsa\"}"}}]}}]}`))
	assert.False(t, gate.Observe(`{"choices":[{"index":0,"delta":{"content":"still streaming"}}]}`))
	assert.True(t, gate.Observe(`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`))

	pending, auditErr := gate.Finish()
	assert.Nil(t, pending)
	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestResponsesStreamGateCorrelatesNameAndArgumentDeltasByOutputIndex(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
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
	})

	gate := NewToolCallAuditStreamGate(nil, 1, "model", ToolCallAuditStreamResponses)
	require.True(t, gate.Observe(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc1","call_id":"c1","name":"shell"}}`))
	require.True(t, gate.Observe(`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc1","delta":"{\"path\":\"~/.ssh/id_rsa\"}"}`))

	pending, auditErr := gate.Finish()
	assert.Nil(t, pending)
	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestResponsesStreamGateAuditsArgumentsDoneWithoutDeltas(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
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
	})
	gate := NewToolCallAuditStreamGate(nil, 1, "model", ToolCallAuditStreamResponses)
	require.True(t, gate.Observe(`{"type":"response.function_call_arguments.done","output_index":0,"item_id":"fc1","name":"shell","arguments":"{\"path\":\"~/.ssh/id_rsa\"}"}`))

	pending, auditErr := gate.Finish()
	assert.Nil(t, pending)
	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestResponsesStreamGateDoesNotDoubleCountDeltaAndDoneArguments(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      32,
		MaxTotalArgumentBytes: 32,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
	})
	gate := NewToolCallAuditStreamGate(nil, 1, "model", ToolCallAuditStreamResponses)
	arguments := `{"cmd":"safe"}`
	require.True(t, gate.Observe(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc1","call_id":"c1","name":"shell"}}`))
	require.True(t, gate.Observe(`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"cmd\":\"safe\"}"}`))
	require.True(t, gate.Observe(`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"fc1","call_id":"c1","name":"shell","arguments":"{\"cmd\":\"safe\"}"}}`))

	pending, auditErr := gate.Finish()
	require.Nil(t, auditErr)
	assert.Len(t, pending, 3)
	assert.LessOrEqual(t, len(arguments), 32)
}

func TestToolCallAuditStreamGateBlocksPendingWireOverLimit(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeAudit,
		MaxArgumentBytes:      16,
		MaxTotalArgumentBytes: 16,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
	})
	gate := NewToolCallAuditStreamGate(nil, 1, "model", ToolCallAuditStreamOpenAIChat)
	hugePadding := strings.Repeat("x", toolCallAuditWireOverhead+32)
	event := fmt.Sprintf(`{"padding":%q,"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"shell","arguments":"{}"}}]}}]}`, hugePadding)
	require.True(t, gate.Observe(event))

	pending, auditErr := gate.Finish()
	assert.Nil(t, pending)
	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestGeminiStreamGateCountsRepeatedCandidatePartCalls(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
		Version:               1,
		Mode:                  setting.ToolCallAuditModeBlock,
		MaxArgumentBytes:      32,
		MaxTotalArgumentBytes: 40,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
	})
	gate := NewToolCallAuditStreamGate(nil, 1, "model", ToolCallAuditStreamGemini)
	event := `{"candidates":[{"index":0,"content":{"parts":[{"functionCall":{"name":"shell","args":{"cmd":"abcdefghijklmnop"}}}]}}]}`
	require.True(t, gate.Observe(event))
	require.True(t, gate.Observe(event))

	pending, auditErr := gate.Finish()
	assert.Nil(t, pending)
	require.NotNil(t, auditErr)
	assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
}

func TestToolCallAuditStreamGateBlocksClaudeAndGeminiCalls(t *testing.T) {
	setToolCallAuditTestSettings(t, setting.ToolCallAuditSettings{
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
	})
	tests := []struct {
		name     string
		protocol string
		events   []string
	}{
		{
			name:     "claude partial json",
			protocol: ToolCallAuditStreamClaude,
			events: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"c1","name":"shell","input":{}}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"~/.ssh/id_rsa\"}"}}`,
			},
		},
		{
			name:     "gemini structured args",
			protocol: ToolCallAuditStreamGemini,
			events: []string{
				`{"candidates":[{"index":0,"content":{"parts":[{"functionCall":{"name":"shell","args":{"path":"~/.ssh/id_rsa"}}}]}}]}`,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gate := NewToolCallAuditStreamGate(nil, 1, "model", test.protocol)
			for _, event := range test.events {
				require.True(t, gate.Observe(event))
			}
			pending, auditErr := gate.Finish()
			assert.Nil(t, pending)
			require.NotNil(t, auditErr)
			assert.Equal(t, types.ErrorCodeToolCallBlocked, auditErr.GetErrorCode())
		})
	}
}

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
