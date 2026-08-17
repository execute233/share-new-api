package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

// ManualCreditQuota 管理员手动充值：原子增加用户额度并强制 int32 上限。
// 条件 UPDATE 保证并发下不超 MaxQuota-1。
func ManualCreditQuota(userId int, quota int) error {
	if userId <= 0 {
		return errors.New("无效的用户 id")
	}
	if quota <= 0 || quota >= common.MaxQuota {
		return errors.New("充值额度无效")
	}
	maxCurrentQuota := common.MaxQuota - 1 - quota
	if maxCurrentQuota < 0 {
		return errors.New("充值额度超出系统可表示范围")
	}
	result := DB.Model(&User{}).
		Where("id = ? AND quota <= ?", userId, maxCurrentQuota).
		Update("quota", gorm.Expr("quota + ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		syncCreditUserQuotaCache(userId, quota, "manual topup")
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("管理员手动充值 %s", logger.LogQuota(quota)))
		return nil
	}
	var count int64
	if err := DB.Model(&User{}).Where("id = ?", userId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("用户不存在")
	}
	return errors.New("用户额度已达上限")
}
