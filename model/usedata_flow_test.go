package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func seedFlowQuotaData(t *testing.T, quotaData QuotaData) {
	t.Helper()
	require.NoError(t, DB.Create(&quotaData).Error)
}

func seedFlowLookupData(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "east"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "west"}).Error)
	require.NoError(t, DB.Create(&Token{Id: 11, UserId: 1, Key: "sk-primary", Name: "primary"}).Error)
	require.NoError(t, DB.Create(&Token{Id: 22, UserId: 2, Key: "sk-backup", Name: "backup"}).Error)
	require.NoError(t, DB.Delete(&Token{Id: 11}).Error)
}

func TestGetFlowQuotaDataUsesQuotaDataRoleSpecificDimensions(t *testing.T) {
	truncateTables(t)
	seedFlowLookupData(t)

	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		NodeName:  "node-a",
		TokenID:   11,
		UseGroup:  "vip",
		ModelName: "gpt-a",
		ChannelID: 1,
		CreatedAt: 1000,
		Count:     2,
		Quota:     100,
		TokenUsed: 40,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		NodeName:  "node-a",
		TokenID:   11,
		UseGroup:  "vip",
		ModelName: "gpt-a",
		ChannelID: 1,
		CreatedAt: 1100,
		Count:     1,
		Quota:     50,
		TokenUsed: 20,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		NodeName:  "node-a",
		TokenID:   11,
		UseGroup:  "vip",
		ModelName: "gpt-a",
		ChannelID: 2,
		CreatedAt: 1200,
		Count:     1,
		Quota:     25,
		TokenUsed: 10,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    2,
		Username:  "bob",
		NodeName:  "node-b",
		TokenID:   22,
		UseGroup:  "default",
		ModelName: "gpt-b",
		ChannelID: 1,
		CreatedAt: 1300,
		Count:     3,
		Quota:     70,
		TokenUsed: 30,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		ModelName: "legacy",
		CreatedAt: 1400,
		Count:     99,
		Quota:     999,
		TokenUsed: 999,
	})

	rootRows, err := GetFlowQuotaData(900, 2000, "", 0, common.RoleRootUser)
	require.NoError(t, err)
	require.Len(t, rootRows, 3)
	// Token 11 was soft-deleted, so its name is intentionally left empty for the
	// frontend to render a localized "deleted (id)" label instead.
	require.Equal(t, FlowQuotaData{
		UserID:      1,
		Username:    "alice",
		NodeName:    "node-a",
		TokenID:     11,
		TokenName:   "",
		UseGroup:    "vip",
		ChannelID:   1,
		ChannelName: "east",
		ModelName:   "gpt-a",
		TokenUsed:   60,
		Count:       3,
		Quota:       150,
	}, *rootRows[0])
	// A token that still exists resolves to its current name.
	require.Equal(t, 22, rootRows[1].TokenID)
	require.Equal(t, "backup", rootRows[1].TokenName)

	adminRows, err := GetFlowQuotaData(900, 2000, "alice", 0, common.RoleAdminUser)
	require.NoError(t, err)
	require.Len(t, adminRows, 2)
	require.Equal(t, 0, adminRows[0].TokenID)
	require.Empty(t, adminRows[0].TokenName)
	require.Empty(t, adminRows[0].NodeName)
	require.Equal(t, "alice", adminRows[0].Username)
	require.Equal(t, "vip", adminRows[0].UseGroup)
	require.Equal(t, "east", adminRows[0].ChannelName)
	require.Equal(t, 150, adminRows[0].Quota)

	selfRows, err := GetFlowQuotaData(900, 2000, "", 1, common.RoleCommonUser)
	require.NoError(t, err)
	require.Len(t, selfRows, 1)
	require.Empty(t, selfRows[0].Username)
	require.Equal(t, 0, selfRows[0].ChannelID)
	require.Empty(t, selfRows[0].ChannelName)
	require.Empty(t, selfRows[0].TokenName)
	require.Equal(t, "vip", selfRows[0].UseGroup)
	require.Equal(t, 175, selfRows[0].Quota)
}

func TestLogQuotaDataSplitsRowsByUseGroupTokenChannelAndNode(t *testing.T) {
	truncateTables(t)
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	LogQuotaData(QuotaDataLogParams{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-a",
		CreatedAt: 3661,
		UseGroup:  "vip",
		TokenID:   11,
		ChannelID: 1,
		NodeName:  "node-a",
		Quota:     100,
		TokenUsed: 40,
	})
	LogQuotaData(QuotaDataLogParams{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-a",
		CreatedAt: 3700,
		UseGroup:  "vip",
		TokenID:   11,
		ChannelID: 1,
		NodeName:  "node-a",
		Quota:     50,
		TokenUsed: 20,
	})
	LogQuotaData(QuotaDataLogParams{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-a",
		CreatedAt: 3700,
		UseGroup:  "default",
		TokenID:   11,
		ChannelID: 1,
		NodeName:  "node-a",
		Quota:     25,
		TokenUsed: 10,
	})

	SaveQuotaDataCache()

	var rows []QuotaData
	require.NoError(t, DB.Order("quota DESC").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, int64(3600), rows[0].CreatedAt)
	require.Equal(t, "vip", rows[0].UseGroup)
	require.Equal(t, 11, rows[0].TokenID)
	require.Equal(t, 1, rows[0].ChannelID)
	require.Equal(t, "node-a", rows[0].NodeName)
	require.Equal(t, 2, rows[0].Count)
	require.Equal(t, 150, rows[0].Quota)
	require.Equal(t, 60, rows[0].TokenUsed)
	require.Equal(t, "default", rows[1].UseGroup)
	require.Equal(t, 25, rows[1].Quota)
}

func TestQuotaDataCacheTokensDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: dsn}}}
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: dsn, PreferSimpleProtocol: true}}}
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			recorder := &migrationSQLRecorder{}
			db = db.Session(&gorm.Session{Logger: recorder})
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
			versionSQL := "SELECT version()"
			if dialect == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("database version: %s", version)

			for _, source := range []string{"fresh", "before_cache_columns", "release_rc47"} {
				t.Run(source, func(t *testing.T) {
					require.NoError(t, db.Migrator().DropTable(&QuotaData{}))
					t.Cleanup(func() { assert.NoError(t, db.Migrator().DropTable(&QuotaData{})) })
					previousDB := DB
					DB = db
					CacheQuotaDataLock.Lock()
					previousCache := CacheQuotaData
					CacheQuotaData = make(map[string]*QuotaData)
					CacheQuotaDataLock.Unlock()
					t.Cleanup(func() {
						DB = previousDB
						CacheQuotaDataLock.Lock()
						CacheQuotaData = previousCache
						CacheQuotaDataLock.Unlock()
					})
					original := QuotaData{UserID: 1, Username: "alice", ModelName: "gpt-a", CreatedAt: 3600,
						UseGroup: "vip", TokenID: 11, ChannelID: 1, NodeName: "node-a", Count: 1, Quota: 100, TokenUsed: 40}
					if source == "release_rc47" {
						original.CacheReadTokens, original.CacheCreationTokens = 3_000_000_000, 7
					}
					var originalIndexes []gorm.Index
					if source != "fresh" {
						require.NoError(t, db.AutoMigrate(&QuotaData{}))
						if source == "before_cache_columns" {
							// 63cecab82 has the same flow dimensions and indexes, without these two columns.
							for _, column := range []string{"CacheReadTokens", "CacheCreationTokens"} {
								require.NoError(t, db.Migrator().DropColumn(&QuotaData{}, column))
							}
							require.NoError(t, db.Omit("CacheReadTokens", "CacheCreationTokens").Create(&original).Error)
						} else {
							// v1.0.0-rc.47 already has both cache columns and all custom flow dimensions.
							require.NoError(t, db.Create(&original).Error)
						}
						originalIndexes, err = db.Migrator().GetIndexes(&QuotaData{})
						require.NoError(t, err)
					}
					for i := range 2 {
						recorder.reset()
						require.NoError(t, db.AutoMigrate(&QuotaData{}))
						if i > 0 {
							assert.Empty(t, recorder.schemaMutations(), "repeated startup must leave the schema unchanged")
						}
					}
					if source == "fresh" {
						require.NoError(t, db.Omit("CacheReadTokens", "CacheCreationTokens").Create(&original).Error)
					}
					var preserved QuotaData
					require.NoError(t, db.First(&preserved, original.Id).Error)
					assert.Equal(t, original, preserved)
					for _, index := range originalIndexes {
						assert.True(t, db.Migrator().HasIndex(&QuotaData{}, index.Name()), "preserve %s", index.Name())
					}

					params := QuotaDataLogParams{UserID: 1, Username: "alice", ModelName: "gpt-a", CreatedAt: 3661,
						UseGroup: "vip", TokenID: 11, ChannelID: 1, NodeName: "node-a", Quota: 10, TokenUsed: 5,
						CacheReadTokens: 11, CacheCreationTokens: 7}
					for range 2 {
						LogQuotaData(params)
					}
					for i := range 4 {
						variant := params
						variant.Quota, variant.TokenUsed = 1, 2
						variant.CacheReadTokens, variant.CacheCreationTokens = 2, 3
						switch i {
						case 0:
							variant.UseGroup = "default"
						case 1:
							variant.TokenID = 22
						case 2:
							variant.ChannelID = 2
						case 3:
							variant.NodeName = "node-b"
						}
						LogQuotaData(variant)
					}
					SaveQuotaDataCache()
					params.CacheReadTokens, params.CacheCreationTokens = -1, -2
					LogQuotaData(params)
					SaveQuotaDataCache()
					var rows []QuotaData
					require.NoError(t, db.Order("id").Find(&rows).Error)
					require.Len(t, rows, 5, "each flow dimension must keep its own persisted row")
					original.Count, original.Quota, original.TokenUsed = 4, 130, 55
					original.CacheReadTokens += 22
					original.CacheCreationTokens += 14
					assert.Equal(t, original, rows[0], "flush must update the matching row and clamp negative cache counts")
					for _, query := range []struct {
						name string
						run  func() ([]*QuotaData, error)
					}{
						{"username", func() ([]*QuotaData, error) { return GetQuotaDataByUsername("alice", 3600, 7200) }},
						{"user_id", func() ([]*QuotaData, error) { return GetQuotaDataByUserId(1, 3600, 7200) }},
						{"users", func() ([]*QuotaData, error) { return GetQuotaDataGroupByUser(3600, 7200) }},
						{"models", func() ([]*QuotaData, error) { return GetAllQuotaDates(3600, 7200, "") }},
					} {
						t.Run(query.name, func(t *testing.T) {
							got, err := query.run()
							require.NoError(t, err)
							require.Len(t, got, 1)
							assert.Equal(t, 8, got[0].Count)
							assert.Equal(t, 134, got[0].Quota)
							assert.Equal(t, 63, got[0].TokenUsed)
							assert.Equal(t, original.CacheReadTokens+8, got[0].CacheReadTokens)
							assert.Equal(t, original.CacheCreationTokens+12, got[0].CacheCreationTokens)
						})
					}
				})
			}
		})
	}
}
