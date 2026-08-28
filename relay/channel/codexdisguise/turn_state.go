package codexdisguise

import (
	"strings"
	"sync"
	"time"
)

type turnStateEntry struct {
	threadID string
	mintedAt time.Time
}

// turnStateRegistry 记录 x-codex-turn-state 的铸造归属（thread → 铸造方）。
// 多下游共享统一会话时，客户端回带由其他 thread 铸造的 turn-state 是代理链
// 独有的矛盾信号（真实 Codex 不会发生），必须在出站前剥离。
type turnStateRegistry struct {
	mu      sync.Mutex
	entries map[string]turnStateEntry
	ttl     time.Duration
}

func newTurnStateRegistry(ttl time.Duration) *turnStateRegistry {
	return &turnStateRegistry{
		entries: make(map[string]turnStateEntry),
		ttl:     ttl,
	}
}

// note 记录 blob 由指定 thread 铸造（响应侧捕获后调用）。
func (r *turnStateRegistry) note(threadID, blob string) {
	if r == nil || strings.TrimSpace(blob) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	r.entries[blob] = turnStateEntry{threadID: threadID, mintedAt: time.Now()}
}

// guard 返回应出站的 blob：未知 → 记录并放行；同 thread → 放行；异 thread → 剥离（""）。
func (r *turnStateRegistry) guard(threadID, blob string) string {
	blob = strings.TrimSpace(blob)
	if blob == "" {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	entry, ok := r.entries[blob]
	if !ok {
		r.entries[blob] = turnStateEntry{threadID: threadID, mintedAt: time.Now()}
		return blob
	}
	if entry.threadID == threadID {
		return blob
	}
	return ""
}

func (r *turnStateRegistry) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
}

func (r *turnStateRegistry) sweepLocked() {
	if r == nil || r.ttl <= 0 || len(r.entries) == 0 {
		return
	}
	cutoff := time.Now().Add(-r.ttl)
	for blob, entry := range r.entries {
		if entry.mintedAt.Before(cutoff) {
			delete(r.entries, blob)
		}
	}
}