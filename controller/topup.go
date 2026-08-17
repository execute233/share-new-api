package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type AdminManualTopupRequest struct {
	UserId int `json:"user_id"`
	Quota  int `json:"quota"`
}

// AdminCompleteTopUp 管理员手动充值接口（原为支付补单，无支付后改造为直接加额度）
func AdminCompleteTopUp(c *gin.Context) {
	var req AdminManualTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.Quota <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.ManualCreditQuota(req.UserId, req.Quota); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
