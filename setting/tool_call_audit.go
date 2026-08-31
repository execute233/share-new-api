package setting

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const ToolCallAuditOptionKey = "ToolCallAuditSettings"

type ToolCallAuditMode string

const (
	ToolCallAuditModeDisabled ToolCallAuditMode = "disabled"
	ToolCallAuditModeAudit    ToolCallAuditMode = "audit"
	ToolCallAuditModeBlock    ToolCallAuditMode = "block"
)

type ToolCallAuditSettings struct {
	Version               int                 `json:"version"`
	Mode                  ToolCallAuditMode   `json:"mode"`
	ChannelIDs            []int               `json:"channel_ids"`
	Rules                 []ToolCallAuditRule `json:"rules"`
	MaxArgumentBytes      int                 `json:"max_argument_bytes"`
	MaxTotalArgumentBytes int                 `json:"max_total_argument_bytes"`
	MaxJSONDepth          int                 `json:"max_json_depth"`
	LogRetentionDays      int                 `json:"log_retention_days"`
}

type ToolCallAuditRule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Severity      string   `json:"severity"`
	Category      string   `json:"category"`
	ToolNames     []string `json:"tool_names"`
	ArgumentPaths []string `json:"argument_paths"`
	MatchType     string   `json:"match_type"`
	Patterns      []string `json:"patterns"`
}

var toolCallAuditState = struct {
	sync.RWMutex
	config ToolCallAuditSettings
}{config: defaultToolCallAuditSettings()}

var toolCallAuditArgumentPathPattern = regexp.MustCompile(`^[A-Za-z0-9_*-]+(?:\.[A-Za-z0-9_*-]+)*$`)

func defaultToolCallAuditSettings() ToolCallAuditSettings {
	return ToolCallAuditSettings{
		Version:               1,
		Mode:                  ToolCallAuditModeDisabled,
		ChannelIDs:            []int{},
		Rules:                 defaultToolCallAuditRules(),
		MaxArgumentBytes:      64 * 1024,
		MaxTotalArgumentBytes: 256 * 1024,
		MaxJSONDepth:          32,
		LogRetentionDays:      7,
	}
}

func defaultToolCallAuditRules() []ToolCallAuditRule {
	return []ToolCallAuditRule{
		{ID: "credential-access-ssh", Name: "Block SSH private key access", Enabled: true, Severity: "critical", Category: "credential_access", ToolNames: []string{"shell", "read_file", "filesystem_*"}, ArgumentPaths: []string{"cmd", "path", "file", "input"}, MatchType: "contains", Patterns: []string{".ssh", "id_rsa", "id_ed25519", "id_ecdsa"}},
		{ID: "credential-access-env", Name: "Block environment credential access", Enabled: true, Severity: "critical", Category: "credential_access", ToolNames: []string{"shell", "read_file", "filesystem_*"}, ArgumentPaths: []string{"cmd", "path", "file", "input"}, MatchType: "contains", Patterns: []string{".env", ".aws", ".config/gcloud", ".config/azure", "git-credentials"}},
		{ID: "environment-dump", Name: "Block environment dumps", Enabled: true, Severity: "high", Category: "environment_dump", ToolNames: []string{"shell", "exec", "run_*"}, ArgumentPaths: []string{"cmd", "command"}, MatchType: "contains", Patterns: []string{"printenv", "env ", "/proc/"}},
		{ID: "dangerous-shell-exfiltration", Name: "Block shell network exfiltration", Enabled: true, Severity: "critical", Category: "network_exfiltration", ToolNames: []string{"shell", "exec", "run_*"}, ArgumentPaths: []string{"cmd", "command"}, MatchType: "contains", Patterns: []string{"curl", "wget", "--data", "$("}},
		{ID: "encoded-command", Name: "Block encoded shell commands", Enabled: true, Severity: "high", Category: "encoded_command", ToolNames: []string{"shell", "exec", "run_*"}, ArgumentPaths: []string{"cmd", "command"}, MatchType: "contains", Patterns: []string{"powershell", "encodedcommand", "base64"}},
	}
}

func DefaultToolCallAuditRules() []ToolCallAuditRule {
	return cloneToolCallAuditSettings(defaultToolCallAuditSettings()).Rules
}

func GetToolCallAuditSettings() ToolCallAuditSettings {
	toolCallAuditState.RLock()
	defer toolCallAuditState.RUnlock()
	return cloneToolCallAuditSettings(toolCallAuditState.config)
}

func cloneToolCallAuditSettings(input ToolCallAuditSettings) ToolCallAuditSettings {
	input.ChannelIDs = append([]int(nil), input.ChannelIDs...)
	input.Rules = append([]ToolCallAuditRule(nil), input.Rules...)
	for i := range input.Rules {
		input.Rules[i].ToolNames = append([]string(nil), input.Rules[i].ToolNames...)
		input.Rules[i].ArgumentPaths = append([]string(nil), input.Rules[i].ArgumentPaths...)
		input.Rules[i].Patterns = append([]string(nil), input.Rules[i].Patterns...)
	}
	return input
}

func ToolCallAuditSettingsToJSONString() string {
	data, err := common.Marshal(GetToolCallAuditSettings())
	if err != nil {
		return "{}"
	}
	return string(data)
}

func UpdateToolCallAuditSettings(value string) error {
	if value == "" {
		return errors.New("tool call audit settings cannot be empty")
	}
	var next ToolCallAuditSettings
	if err := common.UnmarshalJsonStr(value, &next); err != nil {
		return fmt.Errorf("invalid tool call audit settings: %w", err)
	}
	if err := ValidateToolCallAuditSettings(next); err != nil {
		return err
	}
	toolCallAuditState.Lock()
	toolCallAuditState.config = cloneToolCallAuditSettings(next)
	toolCallAuditState.Unlock()
	return nil
}

func ValidateToolCallAuditSettings(config ToolCallAuditSettings) error {
	if config.Version != 1 {
		return fmt.Errorf("unsupported tool call audit settings version: %d", config.Version)
	}
	if config.Mode != ToolCallAuditModeDisabled && config.Mode != ToolCallAuditModeAudit && config.Mode != ToolCallAuditModeBlock {
		return fmt.Errorf("invalid tool call audit mode: %s", config.Mode)
	}
	if config.MaxArgumentBytes <= 0 || config.MaxArgumentBytes > 1024*1024 {
		return errors.New("max_argument_bytes must be between 1 and 1048576")
	}
	if config.MaxTotalArgumentBytes < config.MaxArgumentBytes || config.MaxTotalArgumentBytes > 4*1024*1024 {
		return errors.New("max_total_argument_bytes is invalid")
	}
	if config.MaxJSONDepth <= 0 || config.MaxJSONDepth > 128 {
		return errors.New("max_json_depth must be between 1 and 128")
	}
	if config.LogRetentionDays <= 0 || config.LogRetentionDays > 3650 {
		return errors.New("log_retention_days must be between 1 and 3650")
	}
	seen := make(map[string]struct{}, len(config.Rules))
	if len(config.Rules) > 256 {
		return errors.New("tool call audit rules cannot exceed 256")
	}
	for _, channelID := range config.ChannelIDs {
		if channelID <= 0 {
			return fmt.Errorf("channel id must be positive: %d", channelID)
		}
	}
	for _, rule := range config.Rules {
		if strings.TrimSpace(rule.ID) == "" || len(rule.ID) > 128 {
			return errors.New("tool call audit rule id is required")
		}
		if len(rule.Name) > 256 {
			return fmt.Errorf("rule %s name is too long", rule.ID)
		}
		switch rule.Severity {
		case "", "low", "medium", "high", "critical":
		default:
			return fmt.Errorf("rule %s has invalid severity", rule.ID)
		}
		switch rule.Category {
		case "", "credential_access", "sensitive_path", "environment_dump", "dangerous_shell", "network_exfiltration", "ssrf", "encoded_command", "reverse_shell", "secret_pattern":
		default:
			return fmt.Errorf("rule %s has invalid category", rule.ID)
		}
		if _, ok := seen[rule.ID]; ok {
			return fmt.Errorf("duplicate tool call audit rule id: %s", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		switch rule.MatchType {
		case "contains", "exact", "glob", "regex", "keyword_set":
		default:
			return fmt.Errorf("unsupported match type for rule %s: %s", rule.ID, rule.MatchType)
		}
		if len(rule.Patterns) == 0 || len(rule.Patterns) > 128 {
			return fmt.Errorf("rule %s must contain at least one pattern", rule.ID)
		}
		if len(rule.ToolNames) > 128 || len(rule.ArgumentPaths) > 128 {
			return fmt.Errorf("rule %s has too many tool names or argument paths", rule.ID)
		}
		for _, toolName := range rule.ToolNames {
			if strings.TrimSpace(toolName) == "" || len(toolName) > 256 {
				return fmt.Errorf("rule %s has an invalid tool name", rule.ID)
			}
		}
		for _, argumentPath := range rule.ArgumentPaths {
			if !toolCallAuditArgumentPathPattern.MatchString(argumentPath) || len(argumentPath) > 512 {
				return fmt.Errorf("rule %s has an invalid argument path", rule.ID)
			}
		}
		for _, pattern := range rule.Patterns {
			if pattern == "" || len(pattern) > 4096 {
				return fmt.Errorf("rule %s contains an oversized pattern", rule.ID)
			}
			if rule.MatchType == "regex" {
				if _, err := regexp.Compile(pattern); err != nil {
					return fmt.Errorf("rule %s has invalid regex: %w", rule.ID, err)
				}
			}
		}
	}
	return nil
}

func ToolCallAuditChannelEnabled(channelID int) bool {
	config := GetToolCallAuditSettings()
	if config.Mode == ToolCallAuditModeDisabled {
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
