package codexdisguise

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type DisguiseKeyType string

const (
	DisguiseKeyTypeSub2API DisguiseKeyType = "sub2api"
	DisguiseKeyTypeOAuth   DisguiseKeyType = "oauth"
	DisguiseKeyTypeAgent   DisguiseKeyType = "agent"
)

type DisguiseKey struct {
	Type            DisguiseKeyType `json:"type"`
	APIKey          string          `json:"api_key,omitempty"`
	AccessToken     string          `json:"access_token,omitempty"`
	RefreshToken    string          `json:"refresh_token,omitempty"`
	AccountID       string          `json:"account_id,omitempty"`
	AgentPrivateKey string          `json:"agent_private_key,omitempty"`
	AgentRuntimeID  string          `json:"agent_runtime_id,omitempty"`
	AgentTaskID     string          `json:"agent_task_id,omitempty"`
}

func ParseDisguiseKey(raw string) (*DisguiseKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("codex disguise channel: empty key")
	}
	var key DisguiseKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("codex disguise channel: key must be a JSON object")
	}
	key.APIKey = strings.TrimSpace(key.APIKey)
	key.AccessToken = strings.TrimSpace(key.AccessToken)
	key.RefreshToken = strings.TrimSpace(key.RefreshToken)
	key.AccountID = strings.TrimSpace(key.AccountID)
	key.AgentPrivateKey = strings.TrimSpace(key.AgentPrivateKey)
	key.AgentRuntimeID = strings.TrimSpace(key.AgentRuntimeID)
	key.AgentTaskID = strings.TrimSpace(key.AgentTaskID)
	if err := key.Validate(); err != nil {
		return nil, err
	}
	return &key, nil
}

func (k *DisguiseKey) Validate() error {
	if k == nil {
		return errors.New("codex disguise channel: nil key")
	}
	switch k.Type {
	case DisguiseKeyTypeSub2API:
		if k.APIKey == "" {
			return errors.New("codex disguise channel: api_key is required for sub2api type")
		}
	case DisguiseKeyTypeOAuth:
		if k.AccessToken == "" && k.RefreshToken == "" {
			return errors.New("codex disguise channel: access_token or refresh_token is required for oauth type")
		}
	case DisguiseKeyTypeAgent:
		if k.AgentPrivateKey == "" || k.AgentRuntimeID == "" || k.AgentTaskID == "" {
			return errors.New("codex disguise channel: agent_private_key, agent_runtime_id and agent_task_id are required for agent type")
		}
	default:
		return errors.New("codex disguise channel: unknown key type: " + string(k.Type))
	}
	return nil
}