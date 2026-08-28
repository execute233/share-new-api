package codexdisguise

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

type FingerprintMode string

const (
	FingerprintModeOff     FingerprintMode = "off"
	FingerprintModeDevice  FingerprintMode = "device"
	FingerprintModeSession FingerprintMode = "session"
	FingerprintModeFull    FingerprintMode = "full"
)

func ParseFingerprintMode(raw string) FingerprintMode {
	switch FingerprintMode(strings.TrimSpace(raw)) {
	case FingerprintModeOff, FingerprintModeDevice, FingerprintModeSession, FingerprintModeFull:
		return FingerprintMode(strings.TrimSpace(raw))
	default:
		return FingerprintModeOff
	}
}

func ValidateFingerprintSeed(seed string) bool {
	trimmed := strings.TrimSpace(seed)
	parsed, err := uuid.Parse(trimmed)
	return err == nil && parsed != uuid.Nil && trimmed == parsed.String()
}

// deriveStableUUIDv4 从种子确定性派生 UUIDv4 格式字符串，同种子永远同值。
func deriveStableUUIDv4(seed string) string {
	h := sha256.Sum256([]byte(seed))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16])
}

type codexFingerprintIDs struct {
	Mode                FingerprintMode
	InstallationID      string
	SessionID           string
	ThreadID            string
	TurnID              string
	WindowID            string
	TurnStartedAtUnixMs int64
}

func resolveCodexFingerprintIDs(mode FingerprintMode, seed, clientSessionID string) *codexFingerprintIDs {
	if mode == FingerprintModeOff || !ValidateFingerprintSeed(seed) {
		return nil
	}
	ids := &codexFingerprintIDs{
		Mode:                mode,
		InstallationID:      deriveStableUUIDv4("new-api:codex-install-id:v2:" + seed),
		TurnStartedAtUnixMs: time.Now().UnixMilli(),
	}
	if mode == FingerprintModeDevice {
		return ids
	}
	ids.SessionID = deriveStableUUIDv4("new-api:codex-session-id:v2:" + seed)
	switch mode {
	case FingerprintModeSession:
		if clientSessionID != "" {
			ids.ThreadID = deriveStableUUIDv4("new-api:codex-thread-id:v2:" + seed + ":" + clientSessionID)
		}
		if ids.ThreadID == "" {
			ids.ThreadID = ids.SessionID
		}
	case FingerprintModeFull:
		ids.ThreadID = ids.SessionID
	}
	ids.TurnID = uuid.Must(uuid.NewV7()).String()
	ids.WindowID = ids.ThreadID + ":0"
	return ids
}

// applyCodexFingerprintHeaders 按收敛 ID 改写出站头。
func applyCodexFingerprintHeaders(h *http.Header, ids *codexFingerprintIDs) {
	if h == nil || ids == nil {
		return
	}
	h.Set("x-codex-installation-id", ids.InstallationID)
	if ids.Mode == FingerprintModeDevice {
		rewriteCodexTurnMetadataFields(h, map[string]any{"installation_id": ids.InstallationID})
		return
	}
	h.Set("x-codex-window-id", ids.WindowID)
	h.Set("x-client-request-id", ids.ThreadID)
	h.Set("session-id", ids.SessionID)
	h.Set("session_id", ids.SessionID)
	h.Set("thread-id", ids.ThreadID)
	rewriteCodexTurnMetadataFields(h, map[string]any{
		"installation_id":         ids.InstallationID,
		"session_id":              ids.SessionID,
		"thread_id":               ids.ThreadID,
		"turn_id":                 ids.TurnID,
		"window_id":               ids.WindowID,
		"turn_started_at_unix_ms": ids.TurnStartedAtUnixMs,
	})
}

func rewriteCodexTurnMetadataFields(h *http.Header, fields map[string]any) {
	raw := strings.TrimSpace(h.Get("x-codex-turn-metadata"))
	if raw == "" {
		return
	}
	var metadata map[string]any
	if err := common.Unmarshal([]byte(raw), &metadata); err != nil || metadata == nil {
		metadata = make(map[string]any, len(fields))
	}
	for k, v := range fields {
		metadata[k] = v
	}
	rebuilt, err := common.Marshal(metadata)
	if err != nil {
		return
	}
	h.Set("x-codex-turn-metadata", string(rebuilt))
}

// applyCodexFingerprintClientMetadata 按收敛 ID 改写请求体 client_metadata 与内嵌 turn-metadata。
func applyCodexFingerprintClientMetadata(reqBody map[string]any, ids *codexFingerprintIDs) bool {
	if reqBody == nil || ids == nil {
		return false
	}
	existing, _ := reqBody["client_metadata"].(map[string]any)
	if existing == nil {
		existing = make(map[string]any)
	}
	modified := false
	if applyCodexFingerprintToClientMetadataMap(existing, ids) {
		reqBody["client_metadata"] = existing
		modified = true
	}
	return modified
}

func applyCodexFingerprintToClientMetadataMap(existing map[string]any, ids *codexFingerprintIDs) bool {
	changed := false
	set := func(k string, v any) {
		if existing[k] != v {
			existing[k] = v
			changed = true
		}
	}
	set("x-codex-installation-id", ids.InstallationID)
	if ids.Mode != FingerprintModeDevice {
		set("session_id", ids.SessionID)
		set("thread_id", ids.ThreadID)
		set("turn_id", ids.TurnID)
		set("x-codex-window-id", ids.WindowID)
		if embedded, ok := existing["x-codex-turn-metadata"].(string); ok && embedded != "" {
			var embeddedMap map[string]any
			if err := common.Unmarshal([]byte(embedded), &embeddedMap); err == nil {
				rewriteCodexTurnMetadataFieldsMap(embeddedMap, ids)
				if rebuilt, err := common.Marshal(embeddedMap); err == nil {
					existing["x-codex-turn-metadata"] = string(rebuilt)
					changed = true
				}
			}
		}
	}
	return changed
}

func rewriteCodexTurnMetadataFieldsMap(m map[string]any, ids *codexFingerprintIDs) {
	fields := map[string]any{
		"installation_id":         ids.InstallationID,
		"session_id":              ids.SessionID,
		"thread_id":               ids.ThreadID,
		"turn_id":                 ids.TurnID,
		"window_id":               ids.WindowID,
		"turn_started_at_unix_ms": ids.TurnStartedAtUnixMs,
	}
	for k, v := range fields {
		m[k] = v
	}
}