package codexdisguise

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

func newTestAgentKey(t *testing.T) agentIdentityKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return agentIdentityKey{runtimeID: "runtime-1", privateKey: priv, taskID: "task-1"}
}

func TestBuildAgentAssertionFormat(t *testing.T) {
	key := newTestAgentKey(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	assertion, err := buildAgentAssertion(key, now)
	require.NoError(t, err)
	require.True(t, len(assertion) > len("AgentAssertion "))
	require.True(t, hasPrefixFold(assertion, "AgentAssertion "))

	encoded := assertion[len("AgentAssertion "):]
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	var envelope map[string]string
	require.NoError(t, json.Unmarshal(raw, &envelope))
	assert.Equal(t, "runtime-1", envelope["agent_runtime_id"])
	assert.Equal(t, "task-1", envelope["task_id"])
	assert.Equal(t, "2026-08-28T12:00:00Z", envelope["timestamp"])

	sig, err := base64.StdEncoding.DecodeString(envelope["signature"])
	require.NoError(t, err)
	payload := []byte("runtime-1:task-1:2026-08-28T12:00:00Z")
	assert.True(t, ed25519.Verify(key.privateKey.Public().(ed25519.PublicKey), payload, sig))
}

func TestBuildAgentAssertionRequiresTask(t *testing.T) {
	key := newTestAgentKey(t)
	key.taskID = ""
	_, err := buildAgentAssertion(key, time.Now())
	require.Error(t, err)
}

func TestSignAgentTaskRegistration(t *testing.T) {
	key := newTestAgentKey(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	formatted, signature, err := signAgentTaskRegistration(key, now)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-28T12:00:00Z", formatted)
	sig, err := base64.StdEncoding.DecodeString(signature)
	require.NoError(t, err)
	payload := []byte("runtime-1:2026-08-28T12:00:00Z")
	assert.True(t, ed25519.Verify(key.privateKey.Public().(ed25519.PublicKey), payload, sig))
}

func TestDecryptAgentTaskIDRoundTrip(t *testing.T) {
	key := newTestAgentKey(t)
	// 用与实现相同的派生路径构造加密方公钥
	seed := key.privateKey.Seed()
	digest := sha512.Sum512(seed)
	var curvePrivate [32]byte
	copy(curvePrivate[:], digest[:32])
	curvePrivate[0] &= 248
	curvePrivate[31] &= 127
	curvePrivate[31] |= 64
	pubBytes, err := curve25519.X25519(curvePrivate[:], curve25519.Basepoint)
	require.NoError(t, err)
	var pub [32]byte
	copy(pub[:], pubBytes)
	plaintext := []byte("task-42")
	sealed, err := box.SealAnonymous(nil, plaintext, &pub, rand.Reader)
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(sealed)

	decrypted, err := decryptAgentTaskID(key, encoded)
	require.NoError(t, err)
	assert.Equal(t, "task-42", decrypted)
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && stringsEqualFold(s[:len(prefix)], prefix)
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}