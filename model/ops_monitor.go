package model

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OpsEvent contains only operational metadata, never request/response bodies or headers.
// ErrorMessage is the original error text, bounded by the collector.
type OpsEvent struct {
	ID             string `json:"id" gorm:"size:64;primaryKey"`
	RequestID      string `json:"request_id" gorm:"size:128;index"`
	Timestamp      int64  `json:"timestamp" gorm:"index:idx_ops_event_time;index:idx_ops_event_channel_time,priority:2"`
	Kind           string `json:"kind" gorm:"size:24"`
	UserID         int    `json:"user_id"`
	Username       string `json:"username" gorm:"size:128"`
	ChannelID      int    `json:"channel_id" gorm:"index:idx_ops_event_channel_time,priority:1"`
	ChannelName    string `json:"channel_name" gorm:"size:128"`
	ModelName      string `json:"model_name" gorm:"size:256"`
	GroupName      string `json:"group_name" gorm:"size:128"`
	Outcome        string `json:"outcome" gorm:"size:24"`
	Status         int    `json:"status"`
	DurationMs     int64  `json:"duration_ms"`
	TTFTMs         *int64 `json:"ttft_ms"`
	Tokens         int64  `json:"tokens"`
	TokensKnown    bool   `json:"tokens_known"`
	Attempt        int    `json:"attempt"`
	ErrorMessage   string `json:"error_message" gorm:"type:text"`
	ErrorTruncated bool   `json:"error_truncated"`
}

type OpsCollection struct {
	ID        int   `json:"-" gorm:"primaryKey"`
	StartedAt int64 `json:"started_at"`
}

// Histograms are mergeable and use upper bounds spaced 15% apart. Percentiles
// are estimates; the average and maximum are exact for collected samples.
type OpsHistogram struct {
	Bins  [160]int64 `json:"bins"`
	Count int64      `json:"count"`
	Sum   int64      `json:"sum"`
	Max   int64      `json:"max"`
}

func (h *OpsHistogram) Add(ms int64) {
	ms = min(max(ms, 0), int64(30*86400*1000))
	i := 0
	if ms > 1 {
		i = min(159, int(math.Ceil(math.Log(float64(ms))/math.Log(1.15))))
	}
	h.Bins[i]++
	h.Count++
	h.Sum += ms
	h.Max = max(h.Max, ms)
}

func (h *OpsHistogram) Merge(other OpsHistogram) {
	for i := range h.Bins {
		h.Bins[i] += other.Bins[i]
	}
	h.Count += other.Count
	h.Sum += other.Sum
	h.Max = max(h.Max, other.Max)
}

type OpsPercentiles struct {
	Count int64  `json:"count"`
	P50   *int64 `json:"p50"`
	P90   *int64 `json:"p90"`
	P95   *int64 `json:"p95"`
	P99   *int64 `json:"p99"`
	Avg   *int64 `json:"avg"`
	Max   *int64 `json:"max"`
}

func (h OpsHistogram) Percentiles() OpsPercentiles {
	p := OpsPercentiles{Count: h.Count}
	if h.Count == 0 {
		return p
	}
	avg := h.Sum / h.Count
	p.Avg, p.Max = &avg, &h.Max
	for _, item := range []struct {
		fraction float64
		dest     **int64
	}{{.5, &p.P50}, {.9, &p.P90}, {.95, &p.P95}, {.99, &p.P99}} {
		target, seen := int64(math.Ceil(float64(h.Count)*item.fraction)), int64(0)
		for i, count := range h.Bins {
			seen += count
			if seen >= target {
				v := min(h.Max, int64(math.Ceil(math.Pow(1.15, float64(i)))))
				*item.dest = &v
				break
			}
		}
	}
	return p
}

type OpsCounts struct {
	Requests      int64 `json:"requests"`
	Success       int64 `json:"success"`
	Failure       int64 `json:"failure"`
	Rejected      int64 `json:"rejected"`
	Cancelled     int64 `json:"cancelled"`
	Throttled     int64 `json:"throttled"`
	Retries       int64 `json:"retries"`
	Tokens        int64 `json:"tokens"`
	UnknownTokens int64 `json:"unknown_tokens"`
}

func (n *OpsCounts) Merge(other OpsCounts) {
	n.Requests += other.Requests
	n.Success += other.Success
	n.Failure += other.Failure
	n.Rejected += other.Rejected
	n.Cancelled += other.Cancelled
	n.Throttled += other.Throttled
	n.Retries += other.Retries
	n.Tokens += other.Tokens
	n.UnknownTokens += other.UnknownTokens
}

type OpsBucket struct {
	ID          string `json:"-" gorm:"size:64;primaryKey"`
	Bucket      int64  `json:"bucket" gorm:"index"`
	Kind        string `json:"kind" gorm:"size:24"`
	ChannelID   int    `json:"channel_id"`
	ChannelName string `json:"channel_name" gorm:"size:128"`
	ModelName   string `json:"model_name" gorm:"size:256"`
	GroupName   string `json:"group_name" gorm:"size:128"`
	OpsCounts   `gorm:"embedded"`
	Duration    string `json:"-" gorm:"type:text"`
	FirstToken  string `json:"-" gorm:"type:text"`
}

// PersistOpsEvents commits details and aggregates together. Stable event IDs
// make retrying a failed/ambiguous batch safe, including task completions.
func PersistOpsEvents(ctx context.Context, events []OpsEvent) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		buckets := map[string]*OpsBucket{}
		durations, firstTokens := map[string]*OpsHistogram{}, map[string]*OpsHistogram{}
		for i := range events {
			e := &events[i]
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(e)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 0 {
				continue
			}
			minute := e.Timestamp / 60 * 60
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%d\x00%s\x00%s", minute, e.Kind, e.ChannelID, e.ModelName, e.GroupName))))
			b := buckets[key]
			if b == nil {
				b = &OpsBucket{}
				err := lockForUpdate(tx).First(b, "id = ?", key).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					*b = OpsBucket{ID: key, Bucket: minute, Kind: e.Kind, ChannelID: e.ChannelID, ModelName: e.ModelName, GroupName: e.GroupName}
				}
				buckets[key] = b
				durations[key], firstTokens[key] = &OpsHistogram{}, &OpsHistogram{}
				if b.Duration != "" {
					if err := common.UnmarshalJsonStr(b.Duration, durations[key]); err != nil {
						return err
					}
				}
				if b.FirstToken != "" {
					if err := common.UnmarshalJsonStr(b.FirstToken, firstTokens[key]); err != nil {
						return err
					}
				}
			}
			b.ChannelName = e.ChannelName
			b.Requests++
			switch e.Outcome {
			case "success":
				b.Success++
			case "failure":
				b.Failure++
			case "cancelled":
				b.Cancelled++
			default:
				b.Rejected++
			}
			if e.Status == 429 || e.Status == 529 {
				b.Throttled++
			}
			if e.Kind == "attempt" && e.Attempt > 1 {
				b.Retries++
			}
			b.Tokens += e.Tokens
			if !e.TokensKnown {
				b.UnknownTokens++
			}
			// Successful requests define latency percentiles; failures/cancellations
			// remain inspectable in details rather than improving the latency metric.
			if e.Outcome == "success" {
				durations[key].Add(e.DurationMs)
				if e.TTFTMs != nil {
					firstTokens[key].Add(*e.TTFTMs)
				}
			}
		}
		for key, b := range buckets {
			d, err := common.Marshal(durations[key])
			if err != nil {
				return err
			}
			b.Duration = string(d)
			f, err := common.Marshal(firstTokens[key])
			if err != nil {
				return err
			}
			b.FirstToken = string(f)
			if err := tx.Save(b).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type OpsFilter struct {
	Start     int64
	End       int64
	ChannelID int
	ModelName string
	GroupName string
	Kind      string
	Outcome   string
	UserID    int
	RequestID string
}

func opsQuery(db *gorm.DB, filter OpsFilter, timeColumn string) *gorm.DB {
	q := db.Where(timeColumn+" >= ? AND "+timeColumn+" < ?", filter.Start, filter.End)
	if filter.ChannelID > 0 {
		q = q.Where("channel_id = ?", filter.ChannelID)
	}
	if filter.ModelName != "" {
		q = q.Where("model_name = ?", filter.ModelName)
	}
	if filter.GroupName != "" {
		q = q.Where("group_name = ?", filter.GroupName)
	}
	if filter.Kind != "" {
		q = q.Where("kind = ?", filter.Kind)
	}
	return q
}

type OpsSummary struct {
	OpsCounts
	Latency OpsPercentiles `json:"latency"`
	TTFT    OpsPercentiles `json:"ttft"`
}

type OpsPoint struct {
	Bucket int64 `json:"bucket"`
	OpsCounts
}

type OpsChannel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	OpsCounts
}

type OpsSnapshot struct {
	StartedAt   int64        `json:"started_at"`
	GeneratedAt int64        `json:"generated_at"`
	Start       int64        `json:"start"`
	End         int64        `json:"end"`
	Step        int64        `json:"step"`
	Summary     OpsSummary   `json:"summary"`
	Attempts    OpsCounts    `json:"attempts"`
	Submissions OpsSummary   `json:"submissions"`
	Tasks       OpsSummary   `json:"tasks"`
	Trend       []OpsPoint   `json:"trend"`
	Channels    []OpsChannel `json:"channels"`
}

func GetOpsSnapshot(ctx context.Context, filter OpsFilter) (*OpsSnapshot, error) {
	db := DB.WithContext(ctx)
	var collection OpsCollection
	if err := db.First(&collection, 1).Error; err != nil {
		return nil, err
	}
	result := &OpsSnapshot{StartedAt: collection.StartedAt, GeneratedAt: time.Now().Unix(), Start: filter.Start, End: filter.End, Step: 60, Trend: []OpsPoint{}, Channels: []OpsChannel{}}
	if filter.End-filter.Start > 6*3600 {
		result.Step = 300
	}
	if filter.End-filter.Start > 2*86400 {
		result.Step = 3600
	}
	var rows []OpsBucket
	if err := opsQuery(db.Model(&OpsBucket{}), filter, "bucket").Limit(50001).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 50000 {
		return nil, errors.New("monitoring range too large; narrow the time range or select a channel/model")
	}
	durations, firstTokens := map[string]OpsHistogram{}, map[string]OpsHistogram{}
	points, channels := map[int64]*OpsPoint{}, map[int]*OpsChannel{}
	for _, row := range rows {
		var summary *OpsSummary
		switch row.Kind {
		case "request":
			summary = &result.Summary
		case "task_submission":
			summary = &result.Submissions
		case "task_completion":
			summary = &result.Tasks
		case "attempt":
			result.Attempts.Merge(row.OpsCounts)
			channel := channels[row.ChannelID]
			if channel == nil {
				channel = &OpsChannel{ID: row.ChannelID, Name: row.ChannelName}
				channels[row.ChannelID] = channel
			}
			channel.Merge(row.OpsCounts)
		}
		if summary != nil {
			summary.Merge(row.OpsCounts)
			var d, f OpsHistogram
			if err := common.UnmarshalJsonStr(row.Duration, &d); err != nil {
				return nil, err
			}
			if err := common.UnmarshalJsonStr(row.FirstToken, &f); err != nil {
				return nil, err
			}
			dh, fh := durations[row.Kind], firstTokens[row.Kind]
			dh.Merge(d)
			fh.Merge(f)
			durations[row.Kind], firstTokens[row.Kind] = dh, fh
		}
		if row.Kind == "request" || row.Kind == "task_submission" || row.Kind == "attempt" {
			ts := row.Bucket / result.Step * result.Step
			point := points[ts]
			if point == nil {
				point = &OpsPoint{Bucket: ts}
				points[ts] = point
			}
			if row.Kind == "attempt" {
				point.Retries += row.Retries
			} else {
				point.Merge(row.OpsCounts)
			}
		}
	}
	for kind, summary := range map[string]*OpsSummary{"request": &result.Summary, "task_submission": &result.Submissions, "task_completion": &result.Tasks} {
		summary.Latency = durations[kind].Percentiles()
		summary.TTFT = firstTokens[kind].Percentiles()
	}
	for ts := filter.Start / result.Step * result.Step; ts < filter.End; ts += result.Step {
		if point := points[ts]; point != nil {
			result.Trend = append(result.Trend, *point)
		} else {
			result.Trend = append(result.Trend, OpsPoint{Bucket: ts})
		}
	}
	for _, channel := range channels {
		result.Channels = append(result.Channels, *channel)
	}
	return result, nil
}

func GetOpsEvents(ctx context.Context, filter OpsFilter, before int64, beforeID string) ([]OpsEvent, error) {
	q := opsQuery(DB.WithContext(ctx).Model(&OpsEvent{}), filter, "timestamp")
	if filter.Outcome != "" {
		q = q.Where("outcome = ?", filter.Outcome)
	}
	if filter.UserID > 0 {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.RequestID != "" {
		q = q.Where("request_id = ?", filter.RequestID)
	}
	if before > 0 {
		q = q.Where("timestamp < ? OR (timestamp = ? AND id < ?)", before, before, beforeID)
	}
	rows := []OpsEvent{}
	err := q.Order("timestamp DESC, id DESC").Limit(101).Find(&rows).Error
	return rows, err
}

func CleanupOpsData(ctx context.Context, now time.Time) error {
	db := DB.WithContext(ctx)
	// Bounded deletes keep retention maintenance from holding a large write lock.
	for _, item := range []struct {
		table  any
		column string
		cutoff int64
	}{{&OpsEvent{}, "timestamp", now.AddDate(0, 0, -7).Unix()}, {&OpsBucket{}, "bucket", now.AddDate(0, 0, -30).Unix()}} {
		var ids []string
		if err := db.Model(item.table).Where(item.column+" < ?", item.cutoff).Limit(1000).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := db.Where("id IN ?", ids).Delete(item.table).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
