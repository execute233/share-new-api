package codexdisguise

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurnStateRegistryUnknownBlobPassesThrough(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-1"
	assert.Equal(t, blob, r.guard("thread-1", blob), "未知 blob 放行并记录")
	entry, ok := r.entries[blob]
	require.True(t, ok)
	assert.Equal(t, "thread-1", entry.threadID)
}

func TestTurnStateRegistrySameThreadPasses(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-2"
	r.guard("thread-1", blob)
	assert.Equal(t, blob, r.guard("thread-1", blob))
}

func TestTurnStateRegistryCrossThreadStrips(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-3"
	r.guard("thread-1", blob)
	assert.Empty(t, r.guard("thread-2", blob), "异 thread 回带剥离")
}

func TestTurnStateRegistryExpiredEntryForgotten(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-4"
	r.guard("thread-1", blob)
	r.entries[blob] = turnStateEntry{threadID: "thread-1", mintedAt: time.Now().Add(-2 * time.Hour)}
	r.sweep()
	_, ok := r.entries[blob]
	assert.False(t, ok, "过期条目被清理")
	assert.Equal(t, blob, r.guard("thread-2", blob), "清理后可重新放行")
}

func TestTurnStateRegistryEmptyBlobIgnored(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	assert.Empty(t, r.guard("thread-1", ""))
	assert.Len(t, r.entries, 0)
}