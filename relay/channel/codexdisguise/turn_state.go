package codexdisguise

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const turnStateRegistrySweepInterval = 256

type turnStateEntry struct {
	threadID string
	mintedAt time.Time
}

// turnStateRegistry 记录 x-codex-turn-state 的铸造归属（渠道+blob → 铸造 thread）。
// 多下游共享统一会话时，客户端回带由其他 thread 铸造的 turn-state 是代理链
// 独有的矛盾信号（真实 Codex 不会发生），必须在出站前剥离。
// 对齐 sub2api 语义：未知回带不插入（防内存 DoS）、写入节流全量清扫、
// 键含渠道 ID（跨渠道互不误伤）。键集有界于上游实际铸造的 blob 数。
type turnStateRegistry struct {
	mu      sync.Mutex
	entries map[string]turnStateEntry
	ttl     time.Duration
	writes  uint64
}

func newTurnStateRegistry(ttl time.Duration) *turnStateRegistry {
	return &turnStateRegistry{
		entries: make(map[string]turnStateEntry),
		ttl:     ttl,
	}
}

func turnStateRegistryKey(channelID int, blob string) string {
	return fmt.Sprintf("%d\x00%s", channelID, blob)
}

// note 记录指定渠道铸造 blob 的 thread（响应侧捕获后调用）。
func (r *turnStateRegistry) note(channelID int, threadID, blob string) {
	if r == nil || strings.TrimSpace(blob) == "" {
		return
	}
	key := turnStateRegistryKey(channelID, blob)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[key] = turnStateEntry{threadID: threadID, mintedAt: time.Now()}
	r.writes++
	if r.writes%turnStateRegistrySweepInterval == 0 {
		r.sweepLocked()
	}
}

// guard 返回应出站的 blob：未知/过期 → 放行（不插入）；同 thread → 放行；异 thread → 剥离（""）。
func (r *turnStateRegistry) guard(channelID int, threadID, blob string) string {
	blob = strings.TrimSpace(blob)
	if blob == "" || r == nil {
		return blob
	}
	key := turnStateRegistryKey(channelID, blob)
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[key]
	if !ok {
		return blob
	}
	if !entry.mintedAt.IsZero() && time.Now().After(entry.mintedAt.Add(r.ttl)) {
		delete(r.entries, key)
		return blob
	}
	if entry.threadID == threadID {
		return blob
	}
	return ""
}

func (r *turnStateRegistry) sweep() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
}

func (r *turnStateRegistry) sweepLocked() {
	if r == nil || r.ttl <= 0 || len(r.entries) == 0 {
		return
	}
	cutoff := time.Now().Add(-r.ttl)
	for key, entry := range r.entries {
		if entry.mintedAt.Before(cutoff) {
			delete(r.entries, key)
		}
	}
}