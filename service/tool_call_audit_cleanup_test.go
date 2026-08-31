package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCleanupExpiredToolCallAuditLogsRemovesOnlyExpiredAuditEntries(t *testing.T) {
	db := setupToolCallAuditLogTestDB(t)
	previousSettings := setting.GetToolCallAuditSettings()
	settings := previousSettings
	settings.LogRetentionDays = 7
	settingsJSON, err := common.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, setting.UpdateToolCallAuditSettings(string(settingsJSON)))
	t.Cleanup(func() {
		previousSettingsJSON, marshalErr := common.Marshal(previousSettings)
		require.NoError(t, marshalErr)
		require.NoError(t, setting.UpdateToolCallAuditSettings(string(previousSettingsJSON)))
	})

	now := time.Unix(1_800_000_000, 0)
	cutoff := now.Add(-7 * 24 * time.Hour).Unix()
	logs := []model.Log{
		{RequestId: "expired-audit", Type: model.LogTypeToolCallAudit, CreatedAt: cutoff - 1},
		{RequestId: "recent-audit", Type: model.LogTypeToolCallAudit, CreatedAt: cutoff},
		{RequestId: "expired-consume", Type: model.LogTypeConsume, CreatedAt: cutoff - 1},
	}
	require.NoError(t, db.Create(&logs).Error)

	require.NoError(t, CleanupExpiredToolCallAuditLogs(now))

	var remaining []model.Log
	require.NoError(t, db.Order("request_id").Find(&remaining).Error)
	remainingRequestIDs := make([]string, 0, len(remaining))
	for _, log := range remaining {
		remainingRequestIDs = append(remainingRequestIDs, log.RequestId)
	}
	assert.Equal(t, []string{"expired-consume", "recent-audit"}, remainingRequestIDs)
}

func setupToolCallAuditLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousLogDB := model.LOG_DB
	previousLogDatabaseType := common.LogDatabaseType()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)

	t.Cleanup(func() {
		model.LOG_DB = previousLogDB
		common.SetLogDatabaseType(previousLogDatabaseType)
		require.NoError(t, sqlDB.Close())
	})
	return db
}
