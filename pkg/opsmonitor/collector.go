package opsmonitor

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const contextKey = "ops_monitor_request"
const MaxErrorBytes = 16384

var ready atomic.Bool
var queue = make(chan model.OpsEvent, 4096)
var dropped atomic.Int64
var lastWrite atomic.Int64
var writeFailed atomic.Bool
var activeMu sync.Mutex
var active = map[string]model.OpsEvent{}

type Request struct {
	mu        sync.Mutex
	started   time.Time
	event     model.OpsEvent
	finished  bool
	hasResult bool
}

func Init() {
	if ready.Swap(true) {
		return
	}
	go writer()
	go sampleSystem()
}

func Begin(c *gin.Context) {
	if !ready.Load() {
		return
	}
	if _, ok := c.Get(contextKey); ok {
		return
	}
	e := model.OpsEvent{ID: uuid.NewString(), RequestID: c.GetString(common.RequestIdKey), Kind: "request"}
	c.Set(contextKey, &Request{started: time.Now(), event: e})
	activeMu.Lock()
	active[e.ID] = e
	activeMu.Unlock()
}

func request(c *gin.Context) *Request {
	value, _ := c.Get(contextKey)
	r, _ := value.(*Request)
	return r
}

func metadata(c *gin.Context, info *relaycommon.RelayInfo) model.OpsEvent {
	e := model.OpsEvent{
		RequestID: c.GetString(common.RequestIdKey), UserID: c.GetInt("id"), Username: c.GetString("username"),
		ChannelID: common.GetContextKeyInt(c, constant.ContextKeyChannelId), ChannelName: common.GetContextKeyString(c, constant.ContextKeyChannelName),
		ModelName: common.GetContextKeyString(c, constant.ContextKeyOriginalModel), GroupName: common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
	}
	if info != nil {
		e.UserID = info.UserId
		e.ModelName = info.OriginModelName
		e.GroupName = info.UsingGroup
		if info.ChannelMeta != nil {
			e.ChannelID = info.ChannelId
		}
		if info.IsStream && info.HasSendResponse() && !info.FirstResponseTime.Before(info.StartTime) {
			ms := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
			e.TTFTMs = &ms
		}
	}
	return e
}

func setOutcome(e *model.OpsEvent, c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	e.Status = http.StatusOK
	e.Outcome = "success"
	if apiErr != nil {
		e.Status = apiErr.StatusCode
		e.ErrorMessage = apiErr.Error()
	}
	if c.Request.Context().Err() == context.Canceled || (apiErr != nil && errors.Is(apiErr, context.Canceled)) {
		e.Outcome = "cancelled"
		return
	}
	if info != nil {
		switch perfmetrics.ClassifyRelayOutcome(c.Request.Context(), info, apiErr) {
		case perfmetrics.OutcomeFailure:
			e.Outcome = "failure"
		case perfmetrics.OutcomeIgnored:
			e.Outcome = "rejected"
		}
		stream := info.StreamStatus.OutcomeSnapshot()
		deadlineExceeded := info.StreamStatus != nil && errors.Is(info.StreamStatus.EndError,context.DeadlineExceeded)
		if !deadlineExceeded && (stream.Response == relaycommon.ResponseOutcomeCancelled || stream.EndReason == relaycommon.StreamEndReasonClientGone || stream.EndReason == relaycommon.StreamEndReasonPingFail) {
			e.Outcome = "cancelled"
		}
		if e.ErrorMessage == "" && info.StreamStatus != nil && info.StreamStatus.EndError != nil {
			e.ErrorMessage = info.StreamStatus.EndError.Error()
		}
		if stream.ErrorStatus > 0 {
			e.Status = stream.ErrorStatus
		}
		if e.ErrorMessage == "" && stream.ErrorCode != "" {
			e.ErrorMessage = stream.ErrorCode
		}
	} else if apiErr != nil {
		if apiErr.StatusCode >= 500 {
			e.Outcome = "failure"
		} else {
			e.Outcome = "rejected"
		}
	}
}

// Result captures the relay's semantic outcome before HTTP error rendering.
// A 200 response containing a failed stream therefore remains a failure.
func Result(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	r := request(c)
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	if info == nil && r.hasResult {
		return
	}
	e := metadata(c, info)
	e.ID, e.Kind = r.event.ID, r.event.Kind
	setOutcome(&e, c, info, apiErr)
	r.event, r.hasResult = e, true
}

func Submission(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError, started time.Time) {
	Result(c, info, apiErr)
	r := request(c)
	if r == nil {
		return
	}
	r.mu.Lock()
	r.event.Kind = "task_submission"
	r.started = started
	r.mu.Unlock()
	Finish(c, "", false)
}

// TaskError preserves local rejection codes, which must not be counted as
// upstream outages simply because the task API uses a different error DTO.
func TaskError(err *dto.TaskError) *types.NewAPIError {
	if err==nil { return nil }
	cause:=err.Error; if cause==nil { cause=errors.New(err.Message) }
	if err.LocalError { return types.NewErrorWithStatusCode(cause,types.ErrorCode(err.Code),err.StatusCode) }
	return types.NewOpenAIError(cause,types.ErrorCode(err.Code),err.StatusCode)
}

func MidjourneyError(response *dto.MidjourneyResponseWithStatusCode, err error) *types.NewAPIError {
	if err!=nil { return types.NewOpenAIError(err,types.ErrorCodeBadResponse,502) }
	if response==nil { return types.NewOpenAIError(errors.New("empty Midjourney response"),types.ErrorCodeBadResponse,502) }
	code:=response.Response.Code
	if response.StatusCode==200 && (code==1 || code==21 || code==22) {
		properties,_:=response.Response.Properties.(map[string]any)
		if properties["status"]!="FAILURE" { return nil }
	}
	cause:=errors.New(response.Response.Description+" "+response.Response.Result)
	if code==24 { return types.NewErrorWithStatusCode(cause,types.ErrorCodeInvalidRequest,400) }
	status:=response.StatusCode; if code==23 { status=429 }
	return types.NewOpenAIError(cause,types.ErrorCodeBadResponse,status)
}

func RecordMidjourney(task *model.Midjourney) {
	if !ready.Load() || task==nil { return }
	name:="mj_"+strings.ToLower(task.Action);if task.Action==constant.MjActionSwapFace{name="swap_face"}
	e:=model.OpsEvent{ID:"mj-"+strconv.Itoa(task.Id),RequestID:task.MjId,Kind:"task_completion",Timestamp:time.Now().Unix(),UserID:task.UserId,ChannelID:task.ChannelId,ModelName:name,Outcome:"success",ErrorMessage:task.FailReason}
	if task.Status=="FAILURE" || task.FailReason!="" { e.Outcome="failure" }
	if task.SubmitTime>0 && task.FinishTime>=task.SubmitTime { e.DurationMs=task.FinishTime-task.SubmitTime }
	Record(e)
}

// Finish also handles pre-relay rejections (authentication, quota, rate limits,
// invalid input) and removes live state even when a handler panics.
func Finish(c *gin.Context, errorBody string, panicked bool) {
	r := request(c)
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	defer func() { activeMu.Lock(); delete(active, r.event.ID); activeMu.Unlock() }()
	e := r.event
	if !r.hasResult {
		e = metadata(c, nil)
		e.ID, e.Kind = r.event.ID, r.event.Kind
		e.Status = c.Writer.Status()
		e.Outcome = "success"
		if e.Status >= 500 {
			e.Outcome = "failure"
		} else if e.Status >= 400 {
			e.Outcome = "rejected"
		}
		if c.Request.Context().Err() == context.Canceled {
			e.Outcome = "cancelled"
		}
	}
	if panicked {
		e.Outcome, e.Status, e.ErrorMessage = "failure", 500, "handler panic"
	}
	if e.ErrorMessage == "" && e.Status >= 400 {
		e.ErrorMessage = errorBody
	}
	e.DurationMs = max(0, time.Since(r.started).Milliseconds())
	e.Tokens = c.GetInt64("ops_tokens")
	e.TokensKnown = c.GetBool("ops_tokens_known") && !c.GetBool("ops_tokens_incomplete")
	e.Timestamp = time.Now().Unix()
	Record(e)
}

func Discard(c *gin.Context) {
	r := request(c)
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finished = true
	activeMu.Lock()
	delete(active, r.event.ID)
	activeMu.Unlock()
}

// BeginAttempt tracks actual relay attempts, not channel-selection failures.
// The returned function must run once even when the upstream call panics.
func BeginAttempt(c *gin.Context, info *relaycommon.RelayInfo, attempt int) func(*types.NewAPIError) {
	if !ready.Load() || request(c) == nil {
		return func(*types.NewAPIError) {}
	}
	started := time.Now()
	e := metadata(c, info)
	e.ID = uuid.NewString()
	e.Kind = "attempt"
	e.Attempt = attempt
	activeMu.Lock()
	active[e.ID] = e
	activeMu.Unlock()
	var once sync.Once
	return func(apiErr *types.NewAPIError) {
		once.Do(func() {
			activeMu.Lock()
			delete(active, e.ID)
			activeMu.Unlock()
			setOutcome(&e, c, info, apiErr)
			e.Timestamp = time.Now().Unix()
			e.DurationMs = max(0, time.Since(started).Milliseconds())
			e.TTFTMs = nil
			Record(e)
		})
	}
}

func RecordTask(task *model.Task, result *relaycommon.TaskInfo) {
	if !ready.Load() || task == nil {
		return
	}
	e := model.OpsEvent{ID: "task-" + strconv.FormatInt(task.ID, 10), RequestID: task.TaskID, Kind: "task_completion", UserID: task.UserId, ChannelID: task.ChannelId, ModelName: task.Properties.OriginModelName, GroupName: task.Group, Outcome: "success", Timestamp: time.Now().Unix(), ErrorMessage: task.FailReason}
	if task.Status == model.TaskStatusFailure {
		e.Outcome = "failure"
	}
	// A task result has no client HTTP status; 0 explicitly means not applicable.
	if task.SubmitTime > 0 && task.FinishTime >= task.SubmitTime {
		e.DurationMs = (task.FinishTime - task.SubmitTime) * 1000
	}
	if result != nil && result.TotalTokens > 0 {
		e.Tokens = int64(result.TotalTokens)
		e.TokensKnown = true
	}
	Record(e)
}

func Record(e model.OpsEvent) {
	if !ready.Load() {
		return
	}
	e.RequestID = bounded(e.RequestID, 128)
	e.Username = bounded(e.Username, 128)
	e.ChannelName = bounded(e.ChannelName, 128)
	e.ModelName = bounded(e.ModelName, 256)
	e.GroupName = bounded(e.GroupName, 128)
	if len(e.ErrorMessage) > MaxErrorBytes {
		e.ErrorMessage = bounded(e.ErrorMessage, MaxErrorBytes)
		e.ErrorTruncated = true
	}
	select {
	case queue <- e:
	default:
		dropped.Add(1)
	}
}

func bounded(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}

func writer() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	cleanup := time.NewTicker(time.Minute)
	defer cleanup.Stop()
	batch := make([]model.OpsEvent, 0, 100)
	for {
		// Failed batches remain in memory and are retried on the next tick. Never
		// drain additional data into an unbounded retry buffer.
		if writeFailed.Load() {
			<-ticker.C
		} else {
			select {
			case e := <-queue:
				batch = append(batch, e)
				if len(batch) < 100 {
					continue
				}
			case <-ticker.C:
			case <-cleanup.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := model.CleanupOpsData(ctx, time.Now()); err != nil {
					common.SysError("ops monitoring cleanup failed: " + err.Error())
				}
				cancel()
			}
		}
		if len(batch) == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		// Completion callbacks have IDs rather than display names. Resolve each
		// distinct user/channel once per batch, using the primary database.
		users, channels := map[int]string{}, map[int]string{}
		for i := range batch {
			e := &batch[i]
			if e.Kind != "task_completion" {
				continue
			}
			if name, ok := users[e.UserID]; ok {
				e.Username = name
			} else {
				_ = model.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", e.UserID).Select("username").Scan(&e.Username).Error
				users[e.UserID] = e.Username
			}
			if name, ok := channels[e.ChannelID]; ok {
				e.ChannelName = name
			} else {
				_ = model.DB.WithContext(ctx).Model(&model.Channel{}).Where("id = ?", e.ChannelID).Select("name").Scan(&e.ChannelName).Error
				channels[e.ChannelID] = e.ChannelName
			}
		}
		err := model.PersistOpsEvents(ctx, batch)
		cancel()
		writeFailed.Store(err != nil)
		if err != nil {
			common.SysError("ops monitoring write failed: " + err.Error())
			continue
		}
		batch = batch[:0]
		lastWrite.Store(time.Now().Unix())
	}
}

func Matches(e model.OpsEvent, f model.OpsFilter) bool {
	return (f.ChannelID == 0 || e.ChannelID == f.ChannelID) && (f.ModelName == "" || e.ModelName == f.ModelName) && (f.GroupName == "" || e.GroupName == f.GroupName)
}

type Live struct {
	InFlight    int           `json:"in_flight"`
	Channels    []LiveChannel `json:"channels"`
	Pending     int           `json:"pending"`
	Dropped     int64         `json:"dropped"`
	LastWrite   int64         `json:"last_write"`
	WriteFailed bool          `json:"write_failed"`
}
type LiveChannel struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	InFlight int    `json:"in_flight"`
}

func LiveSnapshot(f model.OpsFilter) Live {
	l := Live{Channels: []LiveChannel{}, Pending: len(queue), Dropped: dropped.Load(), LastWrite: lastWrite.Load(), WriteFailed: writeFailed.Load()}
	channels := map[int]*LiveChannel{}
	activeMu.Lock()
	defer activeMu.Unlock()
	for _, e := range active {
		if !Matches(e, f) {
			continue
		}
		if e.Kind != "attempt" {
			if f.ChannelID == 0 && f.ModelName == "" && f.GroupName == "" {
				l.InFlight++
			}
			continue
		}
		if f.ChannelID > 0 || f.ModelName != "" || f.GroupName != "" {
			l.InFlight++
		}
		ch := channels[e.ChannelID]
		if ch == nil {
			ch = &LiveChannel{ID: e.ChannelID, Name: e.ChannelName}
			channels[e.ChannelID] = ch
		}
		ch.InFlight++
	}
	for _, ch := range channels {
		l.Channels = append(l.Channels, *ch)
	}
	slices.SortFunc(l.Channels, func(a, b LiveChannel) int { return b.InFlight - a.InFlight })
	return l
}

// IsGeneration excludes retrieval, model listings and the Responses socket
// itself (each response.create is measured in its internal request engine).
func IsGeneration(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet {
		return c.Request.URL.Path == "/v1/realtime"
	}
	if c.Request.Method != http.MethodPost {
		return false
	}
	path := c.Request.URL.Path
	return !strings.Contains(path, "/fetch") && !strings.Contains(path, "/list-by-condition") && !strings.Contains(path, "/notify")
}
