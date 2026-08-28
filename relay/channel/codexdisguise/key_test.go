package codexdisguise

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDisguiseKeySub2API(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"sub2api","api_key":"sk-sub2api-123"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeSub2API, key.Type)
	assert.Equal(t, "sk-sub2api-123", key.APIKey)
}

func TestParseDisguiseKeyOAuth(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"oauth","access_token":"at","account_id":"acc_1"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeOAuth, key.Type)
	assert.Equal(t, "acc_1", key.AccountID)
}

func TestParseDisguiseKeyOAuthRequiresAccessToken(t *testing.T) {
	_, err := ParseDisguiseKey(`{"type":"oauth","refresh_token":"rt","account_id":"acc_1"}`)
	require.Error(t, err)
}

func TestParseDisguiseKeyAgent(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"agent","agent_private_key":"pk","agent_runtime_id":"rid","agent_task_id":"tid"}`)
	require.NoError(t, err)
	require.Equal(t, DisguiseKeyTypeAgent, key.Type)
	assert.Equal(t, "rid", key.AgentRuntimeID)
}

func TestParseDisguiseKeyRejectsNonJSON(t *testing.T) {
	_, err := ParseDisguiseKey("sk-plain-text")
	require.Error(t, err)
}

func TestParseDisguiseKeyRejectsUnknownType(t *testing.T) {
	_, err := ParseDisguiseKey(`{"type":"unknown","api_key":"x"}`)
	require.Error(t, err)
}

func TestParseDisguiseKeyRejectsMissingRequired(t *testing.T) {
	_, err := ParseDisguiseKey(`{"type":"sub2api"}`)
	require.Error(t, err)
	_, err = ParseDisguiseKey(`{"type":"oauth","refresh_token":"rt"}`)
	require.Error(t, err, "oauth 必须 access_token（hot path 不支持 refresh）")
	_, err = ParseDisguiseKey(`{"type":"oauth"}`)
	require.Error(t, err, "oauth 必须 access_token")
	_, err = ParseDisguiseKey(`{"type":"agent","agent_private_key":"pk"}`)
	require.Error(t, err)
}

func TestParseDisguiseKeyTrimsValues(t *testing.T) {
	key, err := ParseDisguiseKey(`{"type":"sub2api","api_key":"  sk-x  "}`)
	require.NoError(t, err)
	assert.Equal(t, "sk-x", key.APIKey)
}