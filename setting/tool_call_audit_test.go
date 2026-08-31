package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validToolCallAuditSettings() ToolCallAuditSettings {
	return ToolCallAuditSettings{
		Version:               1,
		Mode:                  ToolCallAuditModeBlock,
		MaxArgumentBytes:      1024,
		MaxTotalArgumentBytes: 2048,
		MaxJSONDepth:          8,
		LogRetentionDays:      7,
		Rules: []ToolCallAuditRule{
			{ID: "rule", Enabled: true, MatchType: "regex", Patterns: []string{"secret"}},
		},
	}
}

func TestValidateToolCallAuditSettingsRejectsInvalidRegexAndDuplicateIDs(t *testing.T) {
	config := validToolCallAuditSettings()
	config.Rules[0].Patterns = []string{"("}
	require.Error(t, ValidateToolCallAuditSettings(config))

	config = validToolCallAuditSettings()
	config.Rules = append(config.Rules, config.Rules[0])
	assert.Error(t, ValidateToolCallAuditSettings(config))
}

func TestToolCallAuditChannelScopeUsesAllChannelsWhenEmpty(t *testing.T) {
	previous := GetToolCallAuditSettings()
	t.Cleanup(func() {
		toolCallAuditState.Lock()
		toolCallAuditState.config = previous
		toolCallAuditState.Unlock()
	})

	config := validToolCallAuditSettings()
	toolCallAuditState.Lock()
	toolCallAuditState.config = config
	toolCallAuditState.Unlock()
	assert.True(t, ToolCallAuditChannelEnabled(99))

	config.ChannelIDs = []int{2}
	toolCallAuditState.Lock()
	toolCallAuditState.config = config
	toolCallAuditState.Unlock()
	assert.False(t, ToolCallAuditChannelEnabled(1))
	assert.True(t, ToolCallAuditChannelEnabled(2))
}

func TestUpdateToolCallAuditSettingsKeepsPreviousConfigOnValidationError(t *testing.T) {
	previous := GetToolCallAuditSettings()
	err := UpdateToolCallAuditSettings(`{"version":1,"mode":"invalid"}`)
	require.Error(t, err)
	assert.Equal(t, previous, GetToolCallAuditSettings())
}

func TestValidateToolCallAuditSettingsRejectsArrayIndexPaths(t *testing.T) {
	config := validToolCallAuditSettings()
	config.Rules[0].ArgumentPaths = []string{"items[0].cmd"}
	require.Error(t, ValidateToolCallAuditSettings(config))

	config.Rules[0].ArgumentPaths = []string{"items.cmd"}
	assert.NoError(t, ValidateToolCallAuditSettings(config))
}
