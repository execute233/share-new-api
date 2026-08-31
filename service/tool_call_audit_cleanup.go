package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

const toolCallAuditLogCleanupInterval = time.Hour

// StartToolCallAuditLogCleanup removes expired audit entries independently of
// new tool-call blocks. Only the master instance performs cleanup.
func StartToolCallAuditLogCleanup() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		runToolCallAuditLogCleanup(time.Now())
		ticker := time.NewTicker(toolCallAuditLogCleanupInterval)
		defer ticker.Stop()
		for now := range ticker.C {
			runToolCallAuditLogCleanup(now)
		}
	}()
}

func CleanupExpiredToolCallAuditLogs(now time.Time) error {
	retentionDays := setting.GetToolCallAuditSettings().LogRetentionDays
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	return model.DeleteExpiredToolCallAuditLogs(cutoff)
}

func runToolCallAuditLogCleanup(now time.Time) {
	if err := CleanupExpiredToolCallAuditLogs(now); err != nil {
		common.SysError("failed to delete expired tool call audit logs: " + err.Error())
	}
}
