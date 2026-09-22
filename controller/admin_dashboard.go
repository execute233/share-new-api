package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetAdminDashboard(c *gin.Context) {
	start, end, ok := parseFlowQuotaTimeRange(c)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "hour")
	if end <= start || end-start > 31*86400 || start%3600 != 0 || end%3600 != 0 || end > time.Now().Unix()+3600 || (granularity != "hour" && granularity != "day") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid dashboard range (maximum 31 days, whole hours)"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	snapshot, err := model.GetAdminDashboardSnapshot(ctx, start, end, granularity == "day")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": snapshot})
}
