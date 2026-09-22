package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/opsmonitor"
	"github.com/gin-gonic/gin"
)

func parseOpsFilter(c *gin.Context) (model.OpsFilter, bool) {
	f := model.OpsFilter{ModelName: c.Query("model"), GroupName: c.Query("group"), Kind: c.Query("kind"), Outcome: c.Query("outcome"), RequestID: c.Query("request_id")}
	valid := true
	for _, item := range []struct {
		key  string
		dest *int64
	}{{"start", &f.Start}, {"end", &f.End}} {
		value, err := strconv.ParseInt(c.Query(item.key), 10, 64)
		if err != nil {
			valid = false
		}
		*item.dest = value
	}
	for _, item := range []struct {
		key  string
		dest *int
	}{{"channel_id", &f.ChannelID}, {"user_id", &f.UserID}} {
		if raw := c.Query(item.key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				valid = false
			}
			*item.dest = value
		}
	}
	now := time.Now().Unix()
	valid = valid && f.Start >= 0 && f.Start < f.End && f.End-f.Start <= 30*86400 && f.Start%60 == 0 && f.End%60 == 0 && f.End <= now+60 && f.Start >= now-31*86400
	valid = valid && len(f.ModelName) <= 256 && len(f.GroupName) <= 128 && len(f.RequestID) <= 128
	switch f.Kind {
	case "", "request", "attempt", "task_submission", "task_completion":
	default:
		valid = false
	}
	switch f.Outcome {
	case "", "success", "failure", "rejected", "cancelled":
	default:
		valid = false
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid monitoring filters (whole minutes, maximum 30 days)"})
	}
	return f, valid
}

func GetOpsMonitor(c *gin.Context) {
	f, ok := parseOpsFilter(c)
	if !ok {
		return
	}
	// Kind/outcome/user filters apply to details only; summary is a consistent
	// comparison of final requests, attempts and asynchronous task lifecycles.
	f.Kind = ""
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	snapshot, err := model.GetOpsSnapshot(ctx, f)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recentFilter := f
	recentFilter.End = time.Now().Unix() / 60 * 60
	recentFilter.Start = recentFilter.End - 60
	recent, err := model.GetOpsSnapshot(ctx, recentFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"snapshot": snapshot, "recent": recent, "live": opsmonitor.LiveSnapshot(f), "system": opsmonitor.SystemSnapshot()}})
}

func GetOpsMonitorEvents(c *gin.Context) {
	f, ok := parseOpsFilter(c)
	if !ok {
		return
	}
	before := int64(0)
	beforeID := c.Query("before_id")
	if raw := c.Query("before"); raw != "" {
		var err error
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before <= 0 || len(beforeID) == 0 || len(beforeID) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid cursor"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	rows, err := model.GetOpsEvents(ctx, f, before, beforeID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	hasMore := len(rows) > 100
	if hasMore {
		rows = rows[:100]
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": rows, "has_more": hasMore}})
}
