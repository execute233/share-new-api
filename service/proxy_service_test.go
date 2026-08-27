package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProxyServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Proxy{}, &model.Channel{}))
	return db
}

func TestResolveChannelProxyUsesOnlyActiveProxyID(t *testing.T) {
	setupProxyServiceTestDB(t)
	t.Setenv("CRYPTO_SECRET", "proxy-service-test")
	username, password, err := EncryptProxyCredentials("user", "secret")
	require.NoError(t, err)
	active := model.Proxy{
		Name:              "active",
		Protocol:          model.ProxyProtocolHTTP,
		Host:              "127.0.0.1",
		Port:              8080,
		Status:            model.ProxyStatusActive,
		UsernameEncrypted: username,
		PasswordEncrypted: password,
	}
	require.NoError(t, active.Insert())
	inactive := model.Proxy{Name: "inactive", Protocol: model.ProxyProtocolHTTP, Host: "127.0.0.2", Port: 8081, Status: model.ProxyStatusInactive}
	require.NoError(t, inactive.Insert())

	resolved, err := ResolveChannelProxy(&model.Channel{ProxyID: &active.ID})
	require.NoError(t, err)
	assert.Equal(t, "http://user:secret@127.0.0.1:8080", resolved.URL)

	resolved, err = ResolveChannelProxy(&model.Channel{ProxyID: &inactive.ID})
	require.NoError(t, err)
	assert.Empty(t, resolved.URL)

	legacy := `{"proxy":"http://legacy.invalid:3128"}`
	resolved, err = ResolveChannelProxy(&model.Channel{Setting: &legacy})
	require.NoError(t, err)
	assert.Empty(t, resolved.URL)
}
