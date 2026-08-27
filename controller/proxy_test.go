package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProxyControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedis })
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	previousLog := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previous
		model.LOG_DB = previousLog
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Proxy{}, &model.Channel{}, &model.User{}, &model.Log{}))
	return db
}

func TestQuickAddProxiesReturnsPerLineSanitizedResults(t *testing.T) {
	setupProxyControllerTestDB(t)
	t.Setenv("CRYPTO_SECRET", "proxy-controller-test")
	router := gin.New()
	router.POST("/proxy/quick-add", QuickAddProxies)
	body := `{"urls":["http://example.com:8080","bad://secret:password@example.com:1234","http://example.com:8080"]}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/proxy/quick-add", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"created":true`)
	assert.Contains(t, recorder.Body.String(), `"skipped":true`)
	assert.Contains(t, recorder.Body.String(), `"line":2`)
	assert.NotContains(t, recorder.Body.String(), "password")
	var count int64
	require.NoError(t, model.DB.Model(&model.Proxy{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestDeleteProxyClearsChannelBindings(t *testing.T) {
	setupProxyControllerTestDB(t)
	proxy := model.Proxy{Name: "test", Protocol: model.ProxyProtocolHTTP, Host: "example.com", Port: 8080, Status: model.ProxyStatusActive}
	require.NoError(t, proxy.Insert())
	channel := model.Channel{Name: "bound", Key: "key", ProxyID: &proxy.ID}
	require.NoError(t, model.DB.Create(&channel).Error)

	router := gin.New()
	router.DELETE("/proxy/:id", DeleteProxy)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/proxy/%d", proxy.ID), nil)
	router.ServeHTTP(recorder, request)

	assert.Contains(t, recorder.Body.String(), `"bound_channel_count":1`)
	var saved model.Channel
	require.NoError(t, model.DB.First(&saved, channel.Id).Error)
	assert.Nil(t, saved.ProxyID)
}
