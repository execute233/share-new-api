package opsmonitor

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

type System struct {
	Timestamp    int64    `json:"timestamp"`
	CPU          *float64 `json:"cpu"`
	Memory       *float64 `json:"memory"`
	MemoryUsed   uint64   `json:"memory_used"`
	MemoryTotal  uint64   `json:"memory_total"`
	GoMemory     uint64   `json:"go_memory"`
	Goroutines   int      `json:"goroutines"`
	DBOK         *bool    `json:"db_ok"`
	DBOpen       int      `json:"db_open"`
	DBInUse      int      `json:"db_in_use"`
	DBIdle       int      `json:"db_idle"`
	DBMax        int      `json:"db_max"`
	RedisEnabled bool     `json:"redis_enabled"`
	RedisOK      *bool    `json:"redis_ok"`
	RedisTotal   uint32   `json:"redis_total"`
	RedisIdle    uint32   `json:"redis_idle"`
}

var system atomic.Pointer[System]

func SystemSnapshot() *System { return system.Load() }

func sampleSystem() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		s := &System{Timestamp: time.Now().Unix(), Goroutines: runtime.NumGoroutine(), RedisEnabled: common.RedisEnabled}
		if values, err := cpu.Percent(0, false); err == nil && len(values) > 0 {
			s.CPU = &values[0]
		}
		if m, err := mem.VirtualMemory(); err == nil {
			s.Memory = &m.UsedPercent
			s.MemoryUsed = m.Used
			s.MemoryTotal = m.Total
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		s.GoMemory = m.Alloc
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if db, err := model.DB.DB(); err == nil {
			ok := db.PingContext(ctx) == nil
			s.DBOK = &ok
			stats := db.Stats()
			s.DBOpen = stats.OpenConnections
			s.DBInUse = stats.InUse
			s.DBIdle = stats.Idle
			s.DBMax = stats.MaxOpenConnections
		}
		cancel()
		if common.RedisEnabled && common.RDB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			ok := common.RDB.Ping(ctx).Err() == nil
			s.RedisOK = &ok
			cancel()
			stats := common.RDB.PoolStats()
			s.RedisTotal = stats.TotalConns
			s.RedisIdle = stats.IdleConns
		}
		system.Store(s)
		<-ticker.C
	}
}
