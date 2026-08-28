package codexdisguise

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurnStateRegistryUnknownBlobPassesWithoutInsert(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-1"
	assert.Equal(t, blob, r.guard(1, "thread-1", blob), "未知 blob 放行")
	r.mu.Lock()
	defer r.mu.Unlock()
	assert.Len(t, r.entries, 0, "未知 blob 不插入，防内存 DoS")
}

func TestTurnStateRegistrySameThreadPasses(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-2"
	r.note(1, "thread-1", blob)
	assert.Equal(t, blob, r.guard(1, "thread-1", blob))
}

func TestTurnStateRegistryCrossThreadStrips(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-3"
	r.note(1, "thread-1", blob)
	assert.Empty(t, r.guard(1, "thread-2", blob), "异 thread 回带剥离")
}

func TestTurnStateRegistryCrossChannelIsolation(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-4"
	r.note(1, "thread-1", blob)
	assert.Equal(t, blob, r.guard(2, "thread-1", blob), "渠道 2 无该 blob 记录，不误伤放行")
}

func TestTurnStateRegistryExpiredEntryForgotten(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-5"
	r.note(1, "thread-1", blob)
	r.mu.Lock()
	for k := range r.entries {
		r.entries[k] = turnStateEntry{threadID: "thread-1", mintedAt: time.Now().Add(-2 * time.Hour)}
	}
	r.mu.Unlock()
	assert.Equal(t, blob, r.guard(1, "thread-2", blob), "过期条目惰性删除后放行")
	r.mu.Lock()
	defer r.mu.Unlock()
	assert.Len(t, r.entries, 0)
}

func TestTurnStateRegistryEmptyBlobIgnored(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	assert.Empty(t, r.guard(1, "thread-1", ""))
	assert.Len(t, r.entries, 0)
}

func TestTurnStateRegistrySweepThrottled(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	for i := 0; i < 255; i++ {
		r.note(1, fmt.Sprintf("thread-%d", i), fmt.Sprintf("blob-%d", i))
	}
	r.mu.Lock()
	count := len(r.entries)
	r.mu.Unlock()
	assert.Equal(t, 255, count, "255 次写入不触发全量 sweep")
	r.note(1, "thread-256", "blob-256")
	r.mu.Lock()
	count = len(r.entries)
	r.mu.Unlock()
	assert.Equal(t, 256, count, "第 256 次写入触发 sweep 但有效项保留")
}

func TestTurnStateRegistryNoteAndGuardSharedConvergedThread(t *testing.T) {
	r := newTurnStateRegistry(time.Hour)
	blob := "opaque-blob-6"
	// 多下游收敛到统一 thread：A 铸造后 B 回带，同 thread 放行。
	r.note(1, "converged-thread", blob)
	assert.Equal(t, blob, r.guard(1, "converged-thread", blob))
	require.NotNil(t, r)
}