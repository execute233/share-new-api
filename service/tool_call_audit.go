package service

import (
	"crypto/sha256"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const maxToolCallAuditLogArgumentBytes = 64 * 1024

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
	config := setting.GetToolCallAuditSettings()
	return auditToolCallsWithSettings(c, channelID, modelName, config, calls)
}

func auditToolCallsWithSettings(c *gin.Context, channelID int, modelName string, config setting.ToolCallAuditSettings, calls []NormalizedToolCall) *types.NewAPIError {
	if len(calls) == 0 || !toolCallAuditChannelEnabled(config, channelID) {
		return nil
	}
	totalBytes := 0
	results := make([]ToolCallAuditResult, len(calls))
	blocked := false
	for i, call := range calls {
		totalBytes += len(call.RawArguments)
		results[i] = EvaluateToolCallAuditWithSettings(config, channelID, call, totalBytes)
		if results[i].Blocked {
			blocked = true
		}
	}
	if !blocked {
		return nil
	}
	for i, result := range results {
		if result.Blocked {
			RecordToolCallAuditBlock(c, channelID, modelName, config.Version, result, calls[i])
		}
	}
	return toolCallBlockedError()
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
		calls = append(calls, NormalizedToolCallFromString("openai_responses", output.CallId, output.Name, output.ArgumentsString()))
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
	if call.Parsed && exceedsJSONDepth(call.Arguments, config.MaxJSONDepth) {
		result.OverLimit = true
		result.Blocked = true
		result.Matches = []ToolCallAuditMatch{{
			RuleID:   "builtin.json_depth",
			Name:     "Tool call JSON depth limit exceeded",
			Category: "resource_limit",
			Severity: "critical",
			Reason:   "json_depth_exceeded",
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

func exceedsJSONDepth(value any, maxDepth int) bool {
	type pendingValue struct {
		value any
		depth int
	}
	pending := []pendingValue{{value: value, depth: 1}}
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		if current.depth > maxDepth {
			return true
		}
		switch typed := current.value.(type) {
		case map[string]any:
			for _, child := range typed {
				pending = append(pending, pendingValue{value: child, depth: current.depth + 1})
			}
		case []any:
			for _, child := range typed {
				pending = append(pending, pendingValue{value: child, depth: current.depth + 1})
			}
		}
	}
	return false
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

func RecordToolCallAuditBlock(c *gin.Context, channelID int, modelName string, policyVersion int, result ToolCallAuditResult, call NormalizedToolCall) {
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
		PolicyVersion: policyVersion,
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
	if rule.MatchType == "regex" {
		patterns := make([]*regexp.Regexp, 0, len(rule.Patterns))
		for _, pattern := range rule.Patterns {
			compiled, err := regexp.Compile(pattern)
			if err == nil {
				patterns = append(patterns, compiled)
			}
		}
		for _, value := range values {
			for _, pattern := range patterns {
				if pattern.MatchString(value) {
					return true
				}
			}
		}
		return false
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
		for _, child := range typed {
			values = append(values, argumentValues(child, paths, maxDepth, currentPath)...)
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
		if strings.Contains(pattern, "*") {
			return true
		}
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
