package codexdisguise

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFingerprintMode(t *testing.T) {
	assert.Equal(t, FingerprintModeOff, ParseFingerprintMode(""))
	assert.Equal(t, FingerprintModeOff, ParseFingerprintMode("bogus"))
	assert.Equal(t, FingerprintModeDevice, ParseFingerprintMode("device"))
	assert.Equal(t, FingerprintModeSession, ParseFingerprintMode("session"))
	assert.Equal(t, FingerprintModeFull, ParseFingerprintMode("full"))
}

func TestValidateFingerprintSeed(t *testing.T) {
	assert.True(t, ValidateFingerprintSeed("3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f"))
	assert.False(t, ValidateFingerprintSeed(""))
	assert.False(t, ValidateFingerprintSeed("not-a-uuid"))
	assert.False(t, ValidateFingerprintSeed("3f2e1a9c-8b4d-4f6a-9e1c"), "格式不完整")
	assert.False(t, ValidateFingerprintSeed("3f2e1a9c8b4d4f6a9e1c2d3b4c5d6e7f"), "无连字符不合法")
	assert.False(t, ValidateFingerprintSeed("00000000-0000-0000-0000-000000000000"), "Nil UUID 不合法")
}

func TestDeriveStableUUIDv4Deterministic(t *testing.T) {
	a := deriveStableUUIDv4("seed-1")
	b := deriveStableUUIDv4("seed-1")
	c := deriveStableUUIDv4("seed-2")
	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	require.Len(t, a, 36)
	assert.Equal(t, "4", string(a[14]), "UUIDv4 version 位")
}

func TestResolveFingerprintIDsOffReturnsNil(t *testing.T) {
	assert.Nil(t, resolveCodexFingerprintIDs(FingerprintModeOff, "seed", "client-session"))
	assert.Nil(t, resolveCodexFingerprintIDs(FingerprintModeSession, "", "client-session"))
}

func TestResolveFingerprintIDsSession(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeSession, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "client-session-1")
	require.NotNil(t, ids)
	assert.Equal(t, FingerprintModeSession, ids.Mode)
	assert.NotEmpty(t, ids.InstallationID)
	assert.NotEmpty(t, ids.SessionID)
	assert.NotEmpty(t, ids.ThreadID)
	assert.NotEmpty(t, ids.TurnID)
	assert.Equal(t, ids.ThreadID+":0", ids.WindowID)
	assert.NotEmpty(t, ids.TurnStartedAtUnixMs)
	// 同 seed 同 clientSessionID → 同 thread
	ids2 := resolveCodexFingerprintIDs(FingerprintModeSession, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "client-session-1")
	assert.Equal(t, ids.ThreadID, ids2.ThreadID)
	// 不同 clientSessionID → 不同 thread
	ids3 := resolveCodexFingerprintIDs(FingerprintModeSession, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "client-session-2")
	assert.NotEqual(t, ids.ThreadID, ids3.ThreadID)
}

func TestResolveFingerprintIDsFullThreadEqualsSession(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeFull, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "anything")
	require.NotNil(t, ids)
	assert.Equal(t, ids.SessionID, ids.ThreadID)
}

func TestResolveFingerprintIDsDeviceOnlyInstallation(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeDevice, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "client-session")
	require.NotNil(t, ids)
	assert.NotEmpty(t, ids.InstallationID)
	assert.Empty(t, ids.SessionID)
	assert.Empty(t, ids.ThreadID)
}

func TestApplyFingerprintHeadersSession(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeSession, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "cs")
	require.NotNil(t, ids)
	h := &http.Header{}
	h.Set("session-id", "original")
	h.Set("session_id", "original")
	h.Set("x-codex-turn-metadata", `{"installation_id":"old","session_id":"old","thread_id":"old","turn_id":"old","window_id":"old","turn_started_at_unix_ms":0,"sandbox":"keep"}`)
	applyCodexFingerprintHeaders(h, ids)
	assert.Equal(t, ids.InstallationID, h.Get("x-codex-installation-id"))
	assert.Equal(t, ids.SessionID, h.Get("session-id"))
	assert.Equal(t, ids.SessionID, h.Get("session_id"))
	assert.Equal(t, ids.ThreadID, h.Get("thread-id"))
	assert.Equal(t, ids.ThreadID, h.Get("x-client-request-id"))
	assert.Equal(t, ids.WindowID, h.Get("x-codex-window-id"))
	metadata := h.Get("x-codex-turn-metadata")
	assert.Contains(t, metadata, ids.TurnID)
	assert.Contains(t, metadata, `"sandbox":"keep"`)
}

func TestApplyFingerprintClientMetadata(t *testing.T) {
	ids := resolveCodexFingerprintIDs(FingerprintModeSession, "3f2e1a9c-8b4d-4f6a-9e1c-2d3b4c5d6e7f", "cs")
	require.NotNil(t, ids)
	body := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-installation-id": "old",
			"session_id":              "old",
			"thread_id":               "old",
			"turn_id":                 "old",
			"x-codex-window-id":       "old",
			"x-codex-turn-metadata":   `{"installation_id":"old","turn_id":"old"}`,
		},
	}
	modified := applyCodexFingerprintClientMetadata(body, ids)
	assert.True(t, modified)
	cm, ok := body["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ids.InstallationID, cm["x-codex-installation-id"])
	assert.Equal(t, ids.SessionID, cm["session_id"])
	assert.Equal(t, ids.ThreadID, cm["thread_id"])
	assert.Equal(t, ids.TurnID, cm["turn_id"])
	assert.Contains(t, cm["x-codex-turn-metadata"].(string), ids.TurnID)
}
