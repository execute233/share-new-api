package opsmonitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func TestOpsStorageAcrossDatabases(t *testing.T) {
	previousDB, previousType := model.DB, common.MainDatabaseType()
	t.Cleanup(func(){model.DB=previousDB;common.SetMainDatabaseType(previousType)})
	for _,engine:=range []string{"sqlite","mysql","postgres"} {
		t.Run(engine,func(t *testing.T){
			var dialector gorm.Dialector
			switch engine {
			case "sqlite": dialector=sqlite.Open(filepath.Join(t.TempDir(),"ops.db"))
			case "mysql":
				dsn:=os.Getenv("OPS_TEST_MYSQL_DSN"); if dsn==""{t.Skip("OPS_TEST_MYSQL_DSN is not configured")};require.Contains(t,dsn,"ops_monitor_test");dialector=mysql.Open(dsn)
			case "postgres":
				dsn:=os.Getenv("OPS_TEST_POSTGRES_DSN");if dsn==""{t.Skip("OPS_TEST_POSTGRES_DSN is not configured")};require.Contains(t,dsn,"ops_monitor_test");dialector=postgres.Open(dsn)
			}
			db,err:=gorm.Open(dialector,&gorm.Config{Logger:logger.Default.LogMode(logger.Silent)});require.NoError(t,err)
			sqlDB,err:=db.DB();require.NoError(t,err);t.Cleanup(func(){_ = sqlDB.Close()})
			model.DB=db;common.SetMainDatabaseType(common.DatabaseType(engine))
			// This suite owns only the explicitly named disposable test database.
			require.NoError(t,db.Migrator().DropTable(&model.OpsBucket{},&model.OpsEvent{},&model.OpsCollection{},&model.PerfMetric{},&model.User{}))
			// Existing production models provide a representative pre-feature schema.
			require.NoError(t,db.AutoMigrate(&model.User{},&model.PerfMetric{}))
			user:=model.User{Username:"ops-existing-user",Password:"test-placeholder",Role:common.RoleAdminUser,Status:common.UserStatusEnabled,AffCode:"ops-existing-aff"}
			require.NoError(t,db.Create(&user).Error)
			now:=time.Now().Unix()/60*60
			for range 2 {
				require.NoError(t,db.AutoMigrate(&model.OpsCollection{},&model.OpsEvent{},&model.OpsBucket{}))
				require.NoError(t,db.Clauses(clause.OnConflict{DoNothing:true}).Create(&model.OpsCollection{ID:1,StartedAt:now-3600}).Error)
			}
			var existing model.User;require.NoError(t,db.First(&existing,user.Id).Error);assert.Equal(t,user.Username,existing.Username)
			require.True(t,db.Migrator().HasIndex(&model.PerfMetric{},"idx_perf_model_group_bucket"))
			ttft:=int64(100)
			events:=[]model.OpsEvent{
				{ID:"req-ok",RequestID:"trace-1",Timestamp:now,Kind:"request",ChannelID:1,ChannelName:"one",UserID:user.Id,Username:user.Username,ModelName:"test-model",Outcome:"success",Status:200,DurationMs:1200,TTFTMs:&ttft,Tokens:10,TokensKnown:true},
				{ID:"attempt-error",RequestID:"trace-1",Timestamp:now,Kind:"attempt",ChannelID:2,ModelName:"test-model",Outcome:"failure",Status:429,Attempt:1,ErrorMessage:"raw original upstream error"},
				{ID:"attempt-ok",RequestID:"trace-1",Timestamp:now,Kind:"attempt",ChannelID:1,ModelName:"test-model",Outcome:"success",Status:200,Attempt:2},
				{ID:"task-submit",Timestamp:now,Kind:"task_submission",ChannelID:1,Outcome:"success",DurationMs:20},
				{ID:"task-complete",Timestamp:now,Kind:"task_completion",ChannelID:1,Outcome:"failure",DurationMs:60000},
				{ID:"rejected",Timestamp:now,Kind:"request",ChannelID:1,Outcome:"rejected",Status:403},
				{ID:"cancelled",Timestamp:now,Kind:"request",ChannelID:1,Outcome:"cancelled",Status:499},
				{ID:"unknown-usage",Timestamp:now,Kind:"request",ChannelID:1,ModelName:"test-model",Outcome:"success",DurationMs:800},
			}
			require.NoError(t,model.PersistOpsEvents(context.Background(),events[:2]))
			require.NoError(t,model.PersistOpsEvents(context.Background(),events)) // replay must not double-count
			filter:=model.OpsFilter{Start:now,End:now+60}
			snapshot,err:=model.GetOpsSnapshot(context.Background(),filter);require.NoError(t,err)
			assert.EqualValues(t,4,snapshot.Summary.Requests);assert.EqualValues(t,2,snapshot.Summary.Success)
			assert.EqualValues(t,0,snapshot.Summary.Failure);assert.EqualValues(t,1,snapshot.Summary.Rejected);assert.EqualValues(t,1,snapshot.Summary.Cancelled)
			assert.EqualValues(t,2,snapshot.Attempts.Requests);assert.EqualValues(t,1,snapshot.Attempts.Failure);assert.EqualValues(t,1,snapshot.Attempts.Throttled);assert.EqualValues(t,1,snapshot.Attempts.Retries)
			assert.EqualValues(t,1,snapshot.Submissions.Requests);assert.EqualValues(t,1,snapshot.Tasks.Failure)
			require.NotNil(t,snapshot.Summary.Latency.Avg);assert.EqualValues(t,1000,*snapshot.Summary.Latency.Avg)
			assert.EqualValues(t,2,snapshot.Summary.Latency.Count);assert.EqualValues(t,1,snapshot.Summary.TTFT.Count)
			assert.Nil(t,snapshot.Tasks.Latency.P99)
			assert.EqualValues(t,10,snapshot.Summary.Tokens)
			filter.Kind="attempt";filter.RequestID="trace-1"
			rows,err:=model.GetOpsEvents(context.Background(),filter,0,"");require.NoError(t,err);require.Len(t,rows,2)
			assert.Equal(t,"raw original upstream error",rows[1].ErrorMessage)
			next,err:=model.GetOpsEvents(context.Background(),filter,rows[0].Timestamp,rows[0].ID);require.NoError(t,err);require.Len(t,next,1);assert.Equal(t,rows[1].ID,next[0].ID)
			filter.ChannelID=1
			rows,err=model.GetOpsEvents(context.Background(),filter,0,"");require.NoError(t,err);require.Len(t,rows,1);assert.Equal(t,"attempt-ok",rows[0].ID)
			old:=model.OpsEvent{ID:"old-detail",Timestamp:now-8*86400,Kind:"request",Outcome:"success"}
			expired:=model.OpsEvent{ID:"expired",Timestamp:now-31*86400,Kind:"request",Outcome:"success"}
			require.NoError(t,model.PersistOpsEvents(context.Background(),[]model.OpsEvent{old,expired}))
			require.NoError(t,model.CleanupOpsData(context.Background(),time.Unix(now,0)))
			var count int64;require.NoError(t,db.Model(&model.OpsEvent{}).Where("id IN ?",[]string{old.ID,expired.ID}).Count(&count).Error);assert.Zero(t,count)
			require.NoError(t,db.Model(&model.OpsBucket{}).Where("bucket = ?",old.Timestamp).Count(&count).Error);assert.EqualValues(t,1,count)
			require.NoError(t,db.Model(&model.OpsBucket{}).Where("bucket = ?",expired.Timestamp).Count(&count).Error);assert.Zero(t,count)
			// A second startup after collected data must preserve both data and the start marker.
			require.NoError(t,db.AutoMigrate(&model.OpsCollection{},&model.OpsEvent{},&model.OpsBucket{}))
			var collection model.OpsCollection;require.NoError(t,db.First(&collection,1).Error);assert.Equal(t,now-3600,collection.StartedAt)
		})
	}
}

func TestRelayMonitoringSeparatesRetryFromFinalResult(t *testing.T) {
	ready.Store(true);t.Cleanup(func(){ready.Store(false)})
	gin.SetMode(gin.TestMode)
	c,_:=gin.CreateTestContext(httptest.NewRecorder());c.Request=httptest.NewRequest(http.MethodPost,"/v1/chat/completions",nil)
	c.Set(common.RequestIdKey,"trace-retry");c.Set("id",7);c.Set("username","visible-admin-name")
	common.SetContextKey(c,constant.ContextKeyChannelId,3)
	info:=&relaycommon.RelayInfo{UserId:7,OriginModelName:"model",StartTime:time.Now(),ChannelMeta:&relaycommon.ChannelMeta{ChannelId:3}}
	Begin(c)
	finish:=BeginAttempt(c,info,1)
	upstream:=types.NewOpenAIError(assert.AnError,types.ErrorCodeBadResponseStatusCode,429)
	finish(upstream);finish(upstream)
	finish=BeginAttempt(c,info,2);finish(nil)
	Result(c,info,nil);c.Set("ops_tokens",int64(25));c.Set("ops_tokens_known",true)
	Finish(c,"",false);Finish(c,"",false)
	require.Len(t,queue,3)
	first,second,final:=<-queue,<-queue,<-queue
	assert.Equal(t,"failure",first.Outcome);assert.Equal(t,429,first.Status);assert.Equal(t,2,second.Attempt)
	assert.Equal(t,"request",final.Kind);assert.Equal(t,"success",final.Outcome);assert.EqualValues(t,25,final.Tokens)
	assert.Equal(t,"visible-admin-name",final.Username);assert.Empty(t,LiveSnapshot(model.OpsFilter{}).Channels)
	assert.Zero(t,LiveSnapshot(model.OpsFilter{}).InFlight)
}

func TestMonitoringKeepsRawBoundedErrorAndStreamFailure(t *testing.T) {
	ready.Store(true);t.Cleanup(func(){ready.Store(false)})
	c,_:=gin.CreateTestContext(httptest.NewRecorder());c.Request=httptest.NewRequest(http.MethodPost,"/v1/responses",nil)
	info:=&relaycommon.RelayInfo{StartTime:time.Now(),StreamStatus:relaycommon.NewStreamStatus()}
	info.StreamStatus.MarkFailed("provider_error","server_error",502)
	Begin(c);Result(c,info,nil);Finish(c,"",false)
	e:=<-queue;assert.Equal(t,"failure",e.Outcome);assert.Equal(t,502,e.Status)
	raw:="<script>raw error</script> "+strings.Repeat("x",MaxErrorBytes)
	Record(model.OpsEvent{ID:"raw",ErrorMessage:raw})
	e=<-queue;assert.True(t,e.ErrorTruncated);assert.Len(t,e.ErrorMessage,MaxErrorBytes);assert.True(t,strings.HasPrefix(e.ErrorMessage,"<script>raw error</script>"))
}
