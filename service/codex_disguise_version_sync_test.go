package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCodexDisguiseVersionTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalIsMasterNode := common.IsMasterNode
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))
	model.InitOptionMap()

	t.Cleanup(func() {
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		common.IsMasterNode = originalIsMasterNode
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

func TestGetCodexDisguiseClientVersionPrefersOption(t *testing.T) {
	setupCodexDisguiseVersionTestDB(t)
	require.NoError(t, SetCodexDisguiseClientVersion("0.147.0"))
	version, err := GetCodexDisguiseClientVersion(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "0.147.0", version)
}

func TestSetCodexDisguiseClientVersionRejectsInvalid(t *testing.T) {
	setupCodexDisguiseVersionTestDB(t)
	err := SetCodexDisguiseClientVersion("not-a-version; rm -rf")
	require.Error(t, err)
	// 非法值不应写库
	assert.Empty(t, common.OptionMap[CodexDisguiseClientVersionOptionKey])
}

func TestSetCodexDisguiseClientVersionEmptyClears(t *testing.T) {
	setupCodexDisguiseVersionTestDB(t)
	require.NoError(t, SetCodexDisguiseClientVersion("0.147.0"))
	require.NoError(t, SetCodexDisguiseClientVersion(""))
	assert.Empty(t, common.OptionMap[CodexDisguiseClientVersionOptionKey])
	model.InitOptionMap()
}