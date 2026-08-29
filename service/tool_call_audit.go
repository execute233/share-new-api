package service

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const maxToolCallAuditLogArgumentBytes = 64 * 1024
const maxBufferedToolCallAuditStreamBytes = 64 * 1024 * 1024

var lastToolCallAuditCleanup atomic.Int64

type NormalizedToolCall struct {
	Protocol     string
	CallID       string
	ToolName     string
	RawArguments []byte
	Arguments    any
	Parsed       bool
}

type ToolCallAuditMatch struct {
	RuleID   string `json:"rule_id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Reason   string `json:"reason"`
}

type ToolCallAuditResult struct {
	Enabled       bool
	Blocked       bool
	OverLimit     bool
	Matches       []ToolCallAuditMatch
	ArgumentsHash string
	ArgumentsSize int
}

// PreAuditUpstreamStream buffers an audited upstream SSE response before any
// bytes reach the client. It reconstructs tool arguments across protocol
// chunks, evaluates the complete calls, and restores the body for the normal
// protocol handler when the stream is allowed.
func PreAuditUpstreamStream(c *gin.Context, channelID int, modelName string, response *http.Response) *types.NewAPIError {
	if response == nil || response.Body == nil || !setting.ToolCallAuditChannelEnabled(channelID) {
		return nil
	}
	originalBody := response.Body
	body, err := io.ReadAll(io.LimitReader(originalBody, maxBufferedToolCallAuditStreamBytes+1))
	_ = originalBody.Close()
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if len(body) > maxBufferedToolCallAuditStreamBytes {
		return types.NewOpenAIError(errors.New("audited upstream stream exceeds safety limit"), types.ErrorCodeToolCallBlocked, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))

	type streamCall struct {
		protocol string
		callID   string
		name     string
		args     strings.Builder
	}
	accumulated := make(map[string]*streamCall)
	completed := make([]NormalizedToolCall, 0)
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), maxBufferedToolCallAuditStreamBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var chat dto.ChatCompletionsStreamResponse
		if common.UnmarshalJsonStr(data, &chat) == nil {
			for _, choice := range chat.Choices {
				for position, tool := range choice.Delta.ToolCalls {
					index := position
					if tool.Index != nil {
						index = *tool.Index
					}
					key := fmt.Sprintf("chat:%d:%d", choice.Index, index)
					entry := accumulated[key]
					if entry == nil {
						entry = &streamCall{protocol: "openai"}
						accumulated[key] = entry
					}
					if tool.ID != "" {
						entry.callID = tool.ID
					}
					if tool.Function.Name != "" {
						entry.name = tool.Function.Name
					}
					entry.args.WriteString(tool.Function.Arguments)
				}
			}
		}

		var responses dto.ResponsesStreamResponse
		if common.UnmarshalJsonStr(data, &responses) == nil {
			key := responses.ItemID
			if key == "" && responses.OutputIndex != nil {
				key = fmt.Sprintf("%d", *responses.OutputIndex)
			}
			if responses.Type == dto.ResponsesOutputTypeItemAdded && responses.Item != nil && responses.Item.Type == dto.BuildInCallFunctionCall {
				entry := accumulated["responses:"+key]
				if entry == nil {
					entry = &streamCall{protocol: "openai_responses"}
					accumulated["responses:"+key] = entry
				}
				entry.callID = responses.Item.CallId
				entry.name = responses.Item.Name
				if len(responses.Item.Arguments) > 0 {
					entry.args.Write(responses.Item.Arguments)
				}
			}
			if responses.Type == "response.function_call_arguments.delta" {
				entry := accumulated["responses:"+key]
				if entry == nil {
					entry = &streamCall{protocol: "openai_responses", callID: responses.ItemID}
					accumulated["responses:"+key] = entry
				}
				entry.args.WriteString(responses.Delta)
			}
			if responses.Type == dto.ResponsesOutputTypeItemDone && responses.Item != nil && responses.Item.Type == dto.BuildInCallFunctionCall {
				entry := accumulated["responses:"+key]
				if entry == nil {
					entry = &streamCall{protocol: "openai_responses"}
					accumulated["responses:"+key] = entry
				}
				entry.callID = responses.Item.CallId
				entry.name = responses.Item.Name
				if entry.args.Len() == 0 && len(responses.Item.Arguments) > 0 {
					entry.args.Write(responses.Item.Arguments)
				}
			}
			if responses.Response != nil {
				for outputIndex, output := range responses.Response.Output {
					if output.Type != dto.BuildInCallFunctionCall {
						continue
					}
					responseKey := output.CallId
					if responseKey == "" {
						responseKey = fmt.Sprintf("completed:%d", outputIndex)
					}
					entry := accumulated["responses:"+responseKey]
					if entry == nil {
						entry = &streamCall{protocol: "openai_responses"}
						accumulated["responses:"+responseKey] = entry
					}
					entry.callID = output.CallId
					entry.name = output.Name
					if entry.args.Len() == 0 && len(output.Arguments) > 0 {
						entry.args.Write(output.Arguments)
					}
				}
			}
		}

		var claude dto.ClaudeResponse
		if common.UnmarshalJsonStr(data, &claude) == nil {
			key := fmt.Sprintf("claude:%d", claude.GetIndex())
			if claude.Type == "content_block_start" && claude.ContentBlock != nil && claude.ContentBlock.Type == "tool_use" {
				accumulated[key] = &streamCall{protocol: "claude", callID: claude.ContentBlock.Id, name: claude.ContentBlock.Name}
			}
			if claude.Type == "content_block_delta" && claude.Delta != nil && claude.Delta.PartialJson != nil {
				entry := accumulated[key]
				if entry == nil {
					entry = &streamCall{protocol: "claude"}
					accumulated[key] = entry
				}
				entry.args.WriteString(*claude.Delta.PartialJson)
			}
		}

		var gemini dto.GeminiChatResponse
		if common.UnmarshalJsonStr(data, &gemini) == nil {
			for candidateIndex, candidate := range gemini.Candidates {
				for partIndex, part := range candidate.Content.Parts {
					if part.FunctionCall == nil {
						continue
					}
					call, marshalErr := NormalizedToolCallFromValue("gemini", fmt.Sprintf("%d:%d", candidateIndex, partIndex), part.FunctionCall.FunctionName, part.FunctionCall.Arguments)
					if marshalErr == nil {
						completed = append(completed, call)
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	for _, entry := range accumulated {
		if entry.name == "" && entry.args.Len() == 0 {
			continue
		}
		completed = append(completed, NormalizedToolCallFromString(entry.protocol, entry.callID, entry.name, entry.args.String()))
	}
	return AuditToolCalls(c, channelID, modelName, completed)
}

func NewNormalizedToolCall(protocol, callID, toolName string, rawArguments []byte) NormalizedToolCall {
	call := NormalizedToolCall{
		Protocol:     protocol,
		CallID:       callID,
		ToolName:     toolName,
		RawArguments: append([]byte(nil), rawArguments...),
	}
	if len(rawArguments) == 0 {
		return call
	}
	var arguments any
	if err := common.Unmarshal(rawArguments, &arguments); err == nil {
		call.Arguments = arguments
		call.Parsed = true
	}
	return call
}

func NormalizedToolCallFromString(protocol, callID, toolName, arguments string) NormalizedToolCall {
	return NewNormalizedToolCall(protocol, callID, toolName, []byte(arguments))
}

func NormalizedToolCallFromValue(protocol, callID, toolName string, arguments any) (NormalizedToolCall, error) {
	raw, err := common.Marshal(arguments)
	if err != nil {
		return NormalizedToolCall{}, err
	}
	return NewNormalizedToolCall(protocol, callID, toolName, raw), nil
}

func AuditToolCalls(c *gin.Context, channelID int, modelName string, calls []NormalizedToolCall) *types.NewAPIError {
	if len(calls) == 0 || !setting.ToolCallAuditChannelEnabled(channelID) {
		return nil
	}
	uniqueCalls := make([]NormalizedToolCall, 0, len(calls))
	seen := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		argumentHash := sha256.Sum256(call.RawArguments)
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%x", call.Protocol, call.CallID, call.ToolName, argumentHash)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniqueCalls = append(uniqueCalls, call)
	}
	totalBytes := 0
	results := make([]ToolCallAuditResult, len(uniqueCalls))
	blocked := false
	for i, call := range uniqueCalls {
		totalBytes += len(call.RawArguments)
		results[i] = EvaluateToolCallAudit(channelID, call, totalBytes)
		if results[i].Blocked {
			blocked = true
		}
	}
	if !blocked {
		return nil
	}
	for i, result := range results {
		if result.Blocked {
			RecordToolCallAuditBlock(c, channelID, modelName, result, uniqueCalls[i])
		}
	}
	return types.NewOpenAIError(errors.New("tool call blocked by security policy"), types.ErrorCodeToolCallBlocked, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}

func AuditOpenAITextResponse(c *gin.Context, channelID int, modelName string, response *dto.OpenAITextResponse) *types.NewAPIError {
	if response == nil {
		return nil
	}
	calls := make([]NormalizedToolCall, 0)
	for _, choice := range response.Choices {
		for _, toolCall := range choice.Message.ParseToolCalls() {
			calls = append(calls, NormalizedToolCallFromString("openai", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments))
		}
	}
	return AuditToolCalls(c, channelID, modelName, calls)
}

func AuditResponsesResponse(c *gin.Context, channelID int, modelName string, response *dto.OpenAIResponsesResponse) *types.NewAPIError {
	if response == nil {
		return nil
	}
	calls := make([]NormalizedToolCall, 0)
	for _, output := range response.Output {
		if output.Type != dto.BuildInCallFunctionCall {
			continue
		}
		calls = append(calls, NewNormalizedToolCall("openai_responses", output.CallId, output.Name, output.Arguments))
	}
	return AuditToolCalls(c, channelID, modelName, calls)
}

func AuditClaudeResponse(c *gin.Context, channelID int, modelName string, response *dto.ClaudeResponse) *types.NewAPIError {
	if response == nil {
		return nil
	}
	calls := make([]NormalizedToolCall, 0)
	for _, block := range response.Content {
		if block.Type != "tool_use" {
			continue
		}
		call, err := NormalizedToolCallFromValue("claude", block.Id, block.Name, block.Input)
		if err != nil {
			call = NewNormalizedToolCall("claude", block.Id, block.Name, nil)
		}
		calls = append(calls, call)
	}
	if response.ContentBlock != nil && response.ContentBlock.Type == "tool_use" {
		call, err := NormalizedToolCallFromValue("claude", response.ContentBlock.Id, response.ContentBlock.Name, response.ContentBlock.Input)
		if err != nil {
			call = NewNormalizedToolCall("claude", response.ContentBlock.Id, response.ContentBlock.Name, nil)
		}
		calls = append(calls, call)
	}
	return AuditToolCalls(c, channelID, modelName, calls)
}

func AuditGeminiResponse(c *gin.Context, channelID int, modelName string, response *dto.GeminiChatResponse) *types.NewAPIError {
	if response == nil {
		return nil
	}
	calls := make([]NormalizedToolCall, 0)
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall == nil {
				continue
			}
			call, err := NormalizedToolCallFromValue("gemini", "", part.FunctionCall.FunctionName, part.FunctionCall.Arguments)
			if err != nil {
				continue
			}
			calls = append(calls, call)
		}
	}
	return AuditToolCalls(c, channelID, modelName, calls)
}

func EvaluateToolCallAudit(channelID int, call NormalizedToolCall, totalArgumentBytes int) ToolCallAuditResult {
	return EvaluateToolCallAuditWithSettings(setting.GetToolCallAuditSettings(), channelID, call, totalArgumentBytes)
}

func EvaluateToolCallAuditWithSettings(config setting.ToolCallAuditSettings, channelID int, call NormalizedToolCall, totalArgumentBytes int) ToolCallAuditResult {
	result := ToolCallAuditResult{
		Enabled:       toolCallAuditChannelEnabled(config, channelID),
		ArgumentsHash: fmt.Sprintf("%x", sha256.Sum256(call.RawArguments)),
		ArgumentsSize: len(call.RawArguments),
	}
	if !result.Enabled {
		return result
	}
	if len(call.RawArguments) > config.MaxArgumentBytes || totalArgumentBytes > config.MaxTotalArgumentBytes {
		result.OverLimit = true
		result.Blocked = true
		result.Matches = []ToolCallAuditMatch{{
			RuleID:   "builtin.argument_limit",
			Name:     "Tool call argument limit exceeded",
			Category: "resource_limit",
			Severity: "critical",
			Reason:   "argument_limit_exceeded",
		}}
		return result
	}

	for _, rule := range config.Rules {
		if !rule.Enabled || !ruleAppliesToTool(rule.ToolNames, call.ToolName) {
			continue
		}
		if ruleMatchesCall(rule, call, config.MaxJSONDepth) {
			result.Matches = append(result.Matches, ToolCallAuditMatch{
				RuleID:   rule.ID,
				Name:     rule.Name,
				Category: rule.Category,
				Severity: rule.Severity,
				Reason:   "rule_match",
			})
		}
	}
	if config.Mode == setting.ToolCallAuditModeBlock && len(result.Matches) > 0 {
		result.Blocked = true
	}
	return result
}

func toolCallAuditChannelEnabled(config setting.ToolCallAuditSettings, channelID int) bool {
	if config.Mode == setting.ToolCallAuditModeDisabled {
		return false
	}
	if len(config.ChannelIDs) == 0 {
		return true
	}
	for _, configuredID := range config.ChannelIDs {
		if configuredID == channelID {
			return true
		}
	}
	return false
}

func TestToolCallAuditSettings(config setting.ToolCallAuditSettings, toolName, rawArguments string) ToolCallAuditResult {
	config.Mode = setting.ToolCallAuditModeBlock
	config.ChannelIDs = nil
	call := NormalizedToolCallFromString("test", "test", toolName, rawArguments)
	return EvaluateToolCallAuditWithSettings(config, 0, call, len(call.RawArguments))
}

func RecordToolCallAuditBlock(c *gin.Context, channelID int, modelName string, result ToolCallAuditResult, call NormalizedToolCall) {
	if c == nil || !result.Blocked {
		return
	}
	ruleIDs := make([]string, 0, len(result.Matches))
	categories := make([]string, 0, len(result.Matches))
	severities := make([]string, 0, len(result.Matches))
	reasons := make([]string, 0, len(result.Matches))
	for _, match := range result.Matches {
		ruleIDs = append(ruleIDs, match.RuleID)
		categories = append(categories, match.Category)
		severities = append(severities, match.Severity)
		reasons = append(reasons, match.Reason)
	}
	now := time.Now()
	lastCleanup := lastToolCallAuditCleanup.Load()
	if now.Unix()-lastCleanup >= int64(time.Hour/time.Second) && lastToolCallAuditCleanup.CompareAndSwap(lastCleanup, now.Unix()) {
		cutoff := now.Add(-time.Duration(setting.GetToolCallAuditSettings().LogRetentionDays) * 24 * time.Hour).Unix()
		if err := model.DeleteExpiredToolCallAuditLogs(cutoff); err != nil {
			common.SysError("failed to delete expired tool call audit logs: " + err.Error())
		}
	}
	loggedArguments := call.RawArguments
	if len(loggedArguments) > maxToolCallAuditLogArgumentBytes {
		loggedArguments = loggedArguments[:maxToolCallAuditLogArgumentBytes]
	}
	model.RecordToolCallAuditLog(model.ToolCallAuditLogParams{
		RequestID:     c.GetString(common.RequestIdKey),
		UserID:        c.GetInt("id"),
		ChannelID:     channelID,
		ModelName:     modelName,
		Protocol:      call.Protocol,
		CallID:        call.CallID,
		ToolName:      call.ToolName,
		Arguments:     string(loggedArguments),
		ArgumentsHash: result.ArgumentsHash,
		ArgumentsSize: result.ArgumentsSize,
		RuleIDs:       ruleIDs,
		Categories:    categories,
		Severities:    severities,
		Reasons:       reasons,
		PolicyVersion: setting.GetToolCallAuditSettings().Version,
	})
}

func ruleMatchesCall(rule setting.ToolCallAuditRule, call NormalizedToolCall, maxDepth int) bool {
	values := make([]string, 0)
	if !call.Parsed || len(rule.ArgumentPaths) == 0 {
		values = append(values, string(call.RawArguments))
	}
	if call.Parsed {
		values = append(values, argumentValues(call.Arguments, rule.ArgumentPaths, maxDepth, "")...)
	}
	for _, value := range values {
		for _, pattern := range rule.Patterns {
			if matchAuditPattern(rule.MatchType, value, pattern) {
				return true
			}
		}
	}
	return false
}

func ruleAppliesToTool(patterns []string, toolName string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if strings.EqualFold(pattern, toolName) {
			return true
		}
		matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(toolName))
		if err == nil && matched {
			return true
		}
	}
	return false
}

func argumentValues(value any, paths []string, maxDepth int, currentPath string) []string {
	if maxDepth > 0 && pathDepth(currentPath) > maxDepth {
		return nil
	}
	if len(paths) > 0 && currentPath != "" && !pathSelected(currentPath, paths) && !pathMayContainSelection(currentPath, paths) {
		return nil
	}
	switch typed := value.(type) {
	case map[string]any:
		values := make([]string, 0)
		for key, child := range typed {
			nextPath := key
			if currentPath != "" {
				nextPath = currentPath + "." + key
			}
			values = append(values, argumentValues(child, paths, maxDepth, nextPath)...)
		}
		return values
	case []any:
		values := make([]string, 0)
		for index, child := range typed {
			values = append(values, argumentValues(child, paths, maxDepth, fmt.Sprintf("%s[%d]", currentPath, index))...)
		}
		return values
	case string:
		if len(paths) == 0 || pathSelected(currentPath, paths) {
			return []string{typed}
		}
	case bool:
		if len(paths) == 0 || pathSelected(currentPath, paths) {
			return []string{fmt.Sprintf("%t", typed)}
		}
	case float64:
		if len(paths) == 0 || pathSelected(currentPath, paths) {
			return []string{fmt.Sprintf("%v", typed)}
		}
	case nil:
		return nil
	default:
		if len(paths) == 0 || pathSelected(currentPath, paths) {
			return []string{fmt.Sprintf("%v", typed)}
		}
	}
	return nil
}

func pathDepth(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, ".") + strings.Count(value, "[") + 1
}

func pathSelected(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.EqualFold(value, pattern) {
			return true
		}
		if matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(value)); err == nil && matched {
			return true
		}
	}
	return false
}

func pathMayContainSelection(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasPrefix(strings.ToLower(pattern), strings.ToLower(value)+".") {
			return true
		}
	}
	return false
}

func matchAuditPattern(matchType, value, pattern string) bool {
	switch matchType {
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	case "exact":
		return strings.EqualFold(value, pattern)
	case "glob":
		matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(value))
		return err == nil && matched
	case "regex":
		matched, err := regexp.MatchString(pattern, value)
		return err == nil && matched
	case "keyword_set":
		for _, word := range setting.SensitiveWords {
			word = strings.TrimSpace(word)
			if word != "" && strings.Contains(strings.ToLower(value), strings.ToLower(word)) {
				return true
			}
		}
	}
	return false
}
