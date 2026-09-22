package model

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DashboardTokens describes disjoint usage categories, independent of prices.
// Unknown/estimated usage must not masquerade as measured zero usage.
type DashboardTokens struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
	Incomplete    bool  `json:"incomplete"`
}

type AdminDashboardCollection struct {
	ID        int `gorm:"primaryKey;autoIncrement:false"`
	StartedAt int64
}

type AdminDashboardBucket struct {
	ID                  int64  `gorm:"primaryKey" json:"-"`
	Bucket              int64  `gorm:"uniqueIndex:idx_admin_dashboard_bucket,priority:1;index" json:"bucket"`
	UserID              int    `gorm:"uniqueIndex:idx_admin_dashboard_bucket,priority:2" json:"user_id"`
	ModelName           string `gorm:"size:128;uniqueIndex:idx_admin_dashboard_bucket,priority:3" json:"model_name"`
	Username            string `gorm:"size:64" json:"username"`
	Requests            int64  `json:"requests"`
	Quota               int64  `json:"quota"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	CacheReadTokens     int64  `json:"cache_read_tokens"`
	IncompleteRequests  int64  `json:"incomplete_requests"`
	DurationSeconds     int64  `json:"duration_seconds"`
	TimedRequests       int64  `json:"timed_requests"`
	UpdatedAt           int64  `gorm:"autoUpdateTime:false" json:"updated_at"`
}

// RecordAdminDashboardUsage persists an atomic hourly increment on the primary
// database. It is independent of log retention and the legacy export interval.
func RecordAdminDashboardUsage(userID int, username, modelName string, quota, duration int, tokens *DashboardTokens, timed bool, requests int64) {
	now := time.Now().Unix()
	row := AdminDashboardBucket{Bucket: now / 3600 * 3600, UserID: userID, Username: username, ModelName: modelName,
		Requests: requests, Quota: int64(quota), UpdatedAt: now}
	if tokens == nil && requests > 0 {
		row.IncompleteRequests = 1
	} else if tokens != nil {
		row.InputTokens, row.OutputTokens = tokens.Input, tokens.Output
		row.CacheCreationTokens, row.CacheReadTokens = tokens.CacheCreation, tokens.CacheRead
		if tokens.Incomplete {
			row.IncompleteRequests = 1
		}
	}
	if timed {
		row.DurationSeconds, row.TimedRequests = int64(max(duration, 0)), 1
	}
	updates := map[string]any{"username": username, "updated_at": now}
	for column, value := range map[string]int64{
		"requests": row.Requests, "quota": row.Quota, "input_tokens": row.InputTokens,
		"output_tokens": row.OutputTokens, "cache_creation_tokens": row.CacheCreationTokens,
		"cache_read_tokens": row.CacheReadTokens, "incomplete_requests": row.IncompleteRequests,
		"duration_seconds": row.DurationSeconds, "timed_requests": row.TimedRequests,
	} {
		updates[column] = gorm.Expr("? + ?", clause.Column{Table: "admin_dashboard_buckets", Name: column}, value)
	}
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bucket"}, {Name: "user_id"}, {Name: "model_name"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&row).Error; err != nil {
		common.SysError("failed to record admin dashboard usage: " + err.Error())
	}
}

const dashboardSums = "COALESCE(SUM(requests),0) requests, COALESCE(SUM(quota),0) quota, COALESCE(SUM(input_tokens),0) input_tokens, COALESCE(SUM(output_tokens),0) output_tokens, COALESCE(SUM(cache_creation_tokens),0) cache_creation_tokens, COALESCE(SUM(cache_read_tokens),0) cache_read_tokens, COALESCE(SUM(incomplete_requests),0) incomplete_requests, COALESCE(SUM(duration_seconds),0) duration_seconds, COALESCE(SUM(timed_requests),0) timed_requests, COALESCE(MAX(updated_at),0) updated_at"

type AdminDashboardSnapshot struct {
	StartedAt      int64                  `json:"started_at"`
	GeneratedAt    int64                  `json:"generated_at"`
	Total          AdminDashboardBucket   `json:"total"`
	Today          AdminDashboardBucket   `json:"today"`
	Keys           int64                  `json:"keys"`
	ActiveKeys     int64                  `json:"active_keys"`
	Channels       int64                  `json:"channels"`
	ActiveChannels int64                  `json:"active_channels"`
	Users          int64                  `json:"users"`
	NewUsers       int64                  `json:"new_users"`
	ActiveUsers    int64                  `json:"active_users"`
	Trend          []AdminDashboardBucket `json:"trend"`
	Models         []AdminDashboardBucket `json:"models"`
	Ranking        []AdminDashboardBucket `json:"ranking"`
	UsersTrend     []AdminDashboardBucket `json:"users_trend"`
}

func GetAdminDashboardSnapshot(ctx context.Context, start, end int64, daily bool) (*AdminDashboardSnapshot, error) {
	now := time.Now().Unix()
	dayStart := (now+8*3600)/86400*86400 - 8*3600
	result := &AdminDashboardSnapshot{GeneratedAt: now}
	db := DB.WithContext(ctx)
	var collection AdminDashboardCollection
	if err := db.First(&collection, 1).Error; err != nil {
		return nil, err
	}
	result.StartedAt = collection.StartedAt
	for _, item := range []struct {
		query *gorm.DB
		dest  any
	}{
		{db.Model(&AdminDashboardBucket{}).Select(dashboardSums), &result.Total},
		{db.Model(&AdminDashboardBucket{}).Where("bucket >= ?", dayStart).Select(dashboardSums), &result.Today},
	} {
		if err := item.query.Scan(item.dest).Error; err != nil {
			return nil, err
		}
	}
	for _, item := range []struct {
		query *gorm.DB
		dest  *int64
	}{
		{db.Model(&Token{}), &result.Keys},
		{db.Model(&Token{}).Where("status = ? AND (expired_time = -1 OR expired_time > ?) AND (unlimited_quota = ? OR remain_quota > 0)", common.TokenStatusEnabled, now, true), &result.ActiveKeys},
		{db.Model(&Channel{}), &result.Channels},
		{db.Model(&Channel{}).Where("status = ?", common.ChannelStatusEnabled), &result.ActiveChannels},
		{db.Model(&User{}), &result.Users},
		{db.Model(&User{}).Where("created_at >= ?", dayStart), &result.NewUsers},
		{db.Model(&AdminDashboardBucket{}).Where("bucket >= ? AND requests > 0", dayStart).Distinct("user_id"), &result.ActiveUsers},
	} {
		if err := item.query.Count(item.dest).Error; err != nil {
			return nil, err
		}
	}
	// Hour-aligned bounds match the storage precision. Daily buckets use UTC+8.
	bucket := "bucket"
	if daily {
		bucket = "((bucket + 28800) / 86400) * 86400 - 28800"
		if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
			bucket = "FLOOR((bucket + 28800) / 86400) * 86400 - 28800"
		}
	}
	query := db.Model(&AdminDashboardBucket{}).Where("bucket >= ? AND bucket < ?", start, end)
	if err := query.Session(&gorm.Session{}).Select(bucket + " AS bucket, " + dashboardSums).Group(bucket).Order("bucket").Scan(&result.Trend).Error; err != nil {
		return nil, err
	}
	if err := query.Session(&gorm.Session{}).Select("model_name, " + dashboardSums).Group("model_name").Order("quota DESC").Limit(20).Scan(&result.Models).Error; err != nil {
		return nil, err
	}
	if err := query.Session(&gorm.Session{}).Select("user_id, MAX(username) username, " + dashboardSums).Group("user_id").Order("quota DESC, user_id").Limit(12).Scan(&result.Ranking).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(result.Ranking))
	for _, user := range result.Ranking {
		ids = append(ids, user.UserID)
	}
	if len(ids) > 0 {
		if err := query.Session(&gorm.Session{}).Where("user_id IN ?", ids).Select(bucket + " AS bucket, user_id, MAX(username) username, " + dashboardSums).Group(bucket + ", user_id").Order("bucket").Scan(&result.UsersTrend).Error; err != nil {
			return nil, err
		}
	}
	return result, nil
}
