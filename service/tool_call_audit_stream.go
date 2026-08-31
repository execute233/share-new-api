package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const (
	ToolCallAuditStreamOpenAIChat = "openai"
	ToolCallAuditStreamResponses  = "openai_responses"
	ToolCallAuditStreamClaude     = "claude"
	ToolCallAuditStreamGemini     = "gemini"
	toolCallAuditWireOverhead     = 64 * 1024
)

type streamToolCall struct {
	protocol        string
	callID          string
	name            string
	args            strings.Builder
	argumentHash    hash.Hash
	argumentSize    int
	loggedArguments []byte
}

type ToolCallAuditStreamGate struct {
	context       *gin.Context
	channelID     int
	modelName     string
	protocol      string
	config        setting.ToolCallAuditSettings
	calls         map[string]*streamToolCall
	callOrder     []string
	pending       []string
	pendingBytes  int
	totalArgBytes int
	overLimit     bool
	overLimitRule string
	overLimitName string
	overLimitWhy  string
	overLimitCall *streamToolCall
	lastCall      *streamToolCall
	geminiCallSeq int
	hasTool       bool
}

func NewToolCallAuditStreamGate(c *gin.Context, channelID int, modelName, protocol string) *ToolCallAuditStreamGate {
	config := setting.GetToolCallAuditSettings()
	if !toolCallAuditChannelEnabled(config, channelID) {
		return nil
	}
	return &ToolCallAuditStreamGate{
		context:   c,
		channelID: channelID,
		modelName: modelName,
		protocol:  protocol,
		config:    config,
		calls:     make(map[string]*streamToolCall),
	}
}

func (g *ToolCallAuditStreamGate) HasTool() bool {
	return g != nil && g.hasTool
}

// Observe returns true when the event must be withheld until Finish audits the
// complete set of logical tool calls in the response.
func (g *ToolCallAuditStreamGate) Observe(data string) bool {
	if g == nil {
		return false
	}
	buffer := false
	switch g.protocol {
	case ToolCallAuditStreamOpenAIChat:
		buffer = g.observeOpenAIChat(data)
	case ToolCallAuditStreamResponses:
		buffer = g.observeResponses(data)
	case ToolCallAuditStreamClaude:
		buffer = g.observeClaude(data)
	case ToolCallAuditStreamGemini:
		buffer = g.observeGemini(data)
	}
	if !buffer {
		return false
	}
	g.hasTool = true
	if g.pendingBytes+len(data) > g.config.MaxTotalArgumentBytes+toolCallAuditWireOverhead {
		g.markOverLimit(g.lastCall, "builtin.stream_buffer_limit", "Tool call stream buffer limit exceeded", "stream_buffer_limit_exceeded")
		return true
	}
	g.pending = append(g.pending, data)
	g.pendingBytes += len(data)
	return true
}

func (g *ToolCallAuditStreamGate) Finish() ([]string, *types.NewAPIError) {
	if g == nil || !g.hasTool {
		return nil, nil
	}
	calls := make([]NormalizedToolCall, 0, len(g.callOrder))
	for _, key := range g.callOrder {
		call := g.calls[key]
		calls = append(calls, NormalizedToolCallFromString(call.protocol, call.callID, call.name, call.args.String()))
	}
	if g.overLimit {
		call := NormalizedToolCall{Protocol: g.protocol}
		resultHash := fmt.Sprintf("%x", sha256.Sum256(nil))
		argumentSize := 0
		if g.overLimitCall != nil {
			call = NormalizedToolCallFromString(g.overLimitCall.protocol, g.overLimitCall.callID, g.overLimitCall.name, string(g.overLimitCall.loggedArguments))
			resultHash = fmt.Sprintf("%x", g.overLimitCall.argumentHash.Sum(nil))
			argumentSize = g.overLimitCall.argumentSize
		} else if len(calls) > 0 {
			call = calls[0]
			resultHash = fmt.Sprintf("%x", sha256.Sum256(call.RawArguments))
			argumentSize = len(call.RawArguments)
		}
		result := ToolCallAuditResult{
			Blocked:       true,
			OverLimit:     true,
			ArgumentsHash: resultHash,
			ArgumentsSize: argumentSize,
			Matches: []ToolCallAuditMatch{{
				RuleID:   g.overLimitRule,
				Name:     g.overLimitName,
				Category: "resource_limit",
				Severity: "critical",
				Reason:   g.overLimitWhy,
			}},
		}
		RecordToolCallAuditBlock(g.context, g.channelID, g.modelName, g.config.Version, result, call)
		return nil, toolCallBlockedError()
	}
	if auditErr := auditToolCallsWithSettings(g.context, g.channelID, g.modelName, g.config, calls); auditErr != nil {
		return nil, auditErr
	}
	return append([]string(nil), g.pending...), nil
}

func (g *ToolCallAuditStreamGate) observeOpenAIChat(data string) bool {
	var response dto.ChatCompletionsStreamResponse
	if common.UnmarshalJsonStr(data, &response) != nil {
		return false
	}
	buffer := false
	for _, choice := range response.Choices {
		if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
			buffer = true
		}
		for position, tool := range choice.Delta.ToolCalls {
			index := position
			if tool.Index != nil {
				index = *tool.Index
			}
			call := g.call(fmt.Sprintf("chat:%d:%d", choice.Index, index), ToolCallAuditStreamOpenAIChat)
			if tool.ID != "" {
				call.callID = tool.ID
			}
			if tool.Function.Name != "" {
				call.name = tool.Function.Name
			}
			g.appendArguments(call, tool.Function.Arguments)
			buffer = true
		}
	}
	return buffer || (g.hasTool && response.Usage != nil)
}

func (g *ToolCallAuditStreamGate) observeResponses(data string) bool {
	var response dto.ResponsesStreamResponse
	if common.UnmarshalJsonStr(data, &response) != nil {
		return false
	}
	key := responsesStreamCallKey(response.OutputIndex, response.ItemID)
	switch response.Type {
	case dto.ResponsesOutputTypeItemAdded:
		if response.Item == nil || response.Item.Type != dto.BuildInCallFunctionCall {
			return false
		}
		call := g.call(key, ToolCallAuditStreamResponses)
		call.callID = response.Item.CallId
		call.name = response.Item.Name
		g.replaceArguments(call, response.Item.ArgumentsString())
		return true
	case "response.function_call_arguments.delta":
		call := g.call(key, ToolCallAuditStreamResponses)
		if call.callID == "" {
			call.callID = response.ItemID
		}
		g.appendArguments(call, response.Delta)
		return true
	case "response.function_call_arguments.done":
		call := g.call(key, ToolCallAuditStreamResponses)
		if response.Name != "" {
			call.name = response.Name
		}
		if response.Arguments != "" {
			g.replaceArguments(call, response.Arguments)
		}
		return true
	case dto.ResponsesOutputTypeItemDone:
		if response.Item == nil || response.Item.Type != dto.BuildInCallFunctionCall {
			return false
		}
		call := g.call(key, ToolCallAuditStreamResponses)
		call.callID = response.Item.CallId
		call.name = response.Item.Name
		if arguments := response.Item.ArgumentsString(); arguments != "" {
			g.replaceArguments(call, arguments)
		}
		return true
	case "response.completed", "response.done", "response.incomplete":
		if response.Response == nil {
			return g.hasTool
		}
		foundTool := false
		for outputIndex := range response.Response.Output {
			output := &response.Response.Output[outputIndex]
			if output.Type != dto.BuildInCallFunctionCall {
				continue
			}
			call := g.call(responsesStreamCallKey(&outputIndex, output.ID), ToolCallAuditStreamResponses)
			call.callID = output.CallId
			call.name = output.Name
			if arguments := output.ArgumentsString(); arguments != "" {
				g.replaceArguments(call, arguments)
			}
			foundTool = true
		}
		return foundTool || g.hasTool
	}
	return false
}

func (g *ToolCallAuditStreamGate) observeClaude(data string) bool {
	var response dto.ClaudeResponse
	if common.UnmarshalJsonStr(data, &response) != nil {
		return false
	}
	key := fmt.Sprintf("claude:%d", response.GetIndex())
	switch response.Type {
	case "content_block_start":
		if response.ContentBlock == nil || response.ContentBlock.Type != "tool_use" {
			return false
		}
		call := g.call(key, ToolCallAuditStreamClaude)
		call.callID = response.ContentBlock.Id
		call.name = response.ContentBlock.Name
		if raw, err := common.Marshal(response.ContentBlock.Input); err == nil {
			arguments := string(raw)
			if arguments != "null" && arguments != "{}" {
				g.replaceArguments(call, arguments)
			}
		}
		return true
	case "content_block_delta":
		if response.Delta == nil || response.Delta.PartialJson == nil {
			return false
		}
		g.appendArguments(g.call(key, ToolCallAuditStreamClaude), *response.Delta.PartialJson)
		return true
	case "content_block_stop":
		_, ok := g.calls[key]
		return ok
	case "message_stop":
		return g.hasTool
	}
	return false
}

func (g *ToolCallAuditStreamGate) observeGemini(data string) bool {
	var response dto.GeminiChatResponse
	if common.UnmarshalJsonStr(data, &response) != nil {
		return false
	}
	foundTool := false
	terminal := false
	for candidateIndex, candidate := range response.Candidates {
		terminal = terminal || candidate.FinishReason != nil
		for partIndex, part := range candidate.Content.Parts {
			if part.FunctionCall == nil {
				continue
			}
			key := fmt.Sprintf("gemini:%d:%d:%d", candidateIndex, partIndex, g.geminiCallSeq)
			g.geminiCallSeq++
			call := g.call(key, ToolCallAuditStreamGemini)
			call.name = part.FunctionCall.FunctionName
			if raw, err := common.Marshal(part.FunctionCall.Arguments); err == nil {
				g.replaceArguments(call, string(raw))
			}
			foundTool = true
		}
	}
	return foundTool || (g.hasTool && terminal)
}

func (g *ToolCallAuditStreamGate) call(key, protocol string) *streamToolCall {
	if call := g.calls[key]; call != nil {
		g.lastCall = call
		return call
	}
	call := &streamToolCall{protocol: protocol, argumentHash: sha256.New()}
	g.calls[key] = call
	g.callOrder = append(g.callOrder, key)
	g.lastCall = call
	return call
}

func (g *ToolCallAuditStreamGate) appendArguments(call *streamToolCall, delta string) {
	if delta == "" {
		return
	}
	call.appendArgumentEvidence(delta)
	if g.overLimit {
		return
	}
	if call.args.Len()+len(delta) > g.config.MaxArgumentBytes || g.totalArgBytes+len(delta) > g.config.MaxTotalArgumentBytes {
		g.markOverLimit(call, "builtin.argument_limit", "Tool call argument limit exceeded", "argument_limit_exceeded")
		return
	}
	call.args.WriteString(delta)
	g.totalArgBytes += len(delta)
}

func (g *ToolCallAuditStreamGate) replaceArguments(call *streamToolCall, arguments string) {
	if arguments == "" {
		return
	}
	call.replaceArgumentEvidence(arguments)
	if g.overLimit {
		return
	}
	nextTotal := g.totalArgBytes - call.args.Len() + len(arguments)
	if len(arguments) > g.config.MaxArgumentBytes || nextTotal > g.config.MaxTotalArgumentBytes {
		g.markOverLimit(call, "builtin.argument_limit", "Tool call argument limit exceeded", "argument_limit_exceeded")
		return
	}
	g.totalArgBytes -= call.args.Len()
	call.args.Reset()
	call.args.WriteString(arguments)
	g.totalArgBytes = nextTotal
}

func (g *ToolCallAuditStreamGate) markOverLimit(call *streamToolCall, ruleID, name, reason string) {
	if g.overLimit {
		return
	}
	g.overLimit = true
	g.overLimitCall = call
	g.overLimitRule = ruleID
	g.overLimitName = name
	g.overLimitWhy = reason
}

func (c *streamToolCall) appendArgumentEvidence(delta string) {
	if c.argumentHash == nil {
		c.argumentHash = sha256.New()
	}
	_, _ = io.WriteString(c.argumentHash, delta)
	c.argumentSize += len(delta)
	remaining := maxToolCallAuditLogArgumentBytes - len(c.loggedArguments)
	if remaining > len(delta) {
		remaining = len(delta)
	}
	if remaining > 0 {
		c.loggedArguments = append(c.loggedArguments, delta[:remaining]...)
	}
}

func (c *streamToolCall) replaceArgumentEvidence(arguments string) {
	c.argumentHash = sha256.New()
	c.argumentSize = 0
	c.loggedArguments = c.loggedArguments[:0]
	c.appendArgumentEvidence(arguments)
}

func responsesStreamCallKey(outputIndex *int, fallback string) string {
	if outputIndex != nil {
		return fmt.Sprintf("responses:%d", *outputIndex)
	}
	return "responses:" + fallback
}

func toolCallBlockedError() *types.NewAPIError {
	return types.NewOpenAIError(errors.New("tool call blocked by security policy"), types.ErrorCodeToolCallBlocked, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}
