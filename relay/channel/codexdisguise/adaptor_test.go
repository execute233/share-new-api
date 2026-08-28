package codexdisguise

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRelayInfo(relayMode int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodexDisguise,
			ChannelBaseUrl: "https://sub2api.example.com",
			ApiKey:         `{"type":"sub2api","api_key":"sk-1"}`,
		},
		RelayMode: relayMode,
	}
}

func TestGetRequestURLResponses(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeResponses))
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example.com/backend-api/codex/responses", url)
}

func TestGetRequestURLCompact(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeResponsesCompact))
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example.com/backend-api/codex/responses/compact", url)
}

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeAlphaSearch))
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example.com/backend-api/codex/alpha/search", url)
}

func TestGetRequestURLRejectsUnsupportedMode(t *testing.T) {
	adaptor := &Adaptor{}
	_, err := adaptor.GetRequestURL(testRelayInfo(relayconstant.RelayModeChatCompletions))
	require.Error(t, err)
}

func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := testRelayInfo(relayconstant.RelayModeResponses)
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            []byte(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: []byte(`1.5`),
		PresencePenalty:  []byte(`1.5`),
	})
	require.NoError(t, err)
	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
	assert.Equal(t, []byte(`false`), []byte(request.Store))
}

func TestConvertOpenAIResponsesRequestDefaultsInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := testRelayInfo(relayconstant.RelayModeResponses)
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model: "gpt-5-codex",
		Input: []byte(`"hello"`),
	})
	require.NoError(t, err)
	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Equal(t, []byte(`""`), []byte(request.Instructions))
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c
}

func TestSetupRequestHeaderSub2APIKey(t *testing.T) {
	c := testGinContext()
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.IsStream = true
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk-1", headers.Get("Authorization"))
	assert.Equal(t, "codex-tui", headers.Get("originator"))
	assert.Equal(t, "0.146.0", headers.Get("version"))
	assert.Contains(t, headers.Get("user-agent"), "codex-tui/0.146.0")
	assert.Equal(t, "text/event-stream", headers.Get("Accept"))
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
}

func TestSetupRequestHeaderOAuthKey(t *testing.T) {
	c := testGinContext()
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelMeta.ApiKey = `{"type":"oauth","access_token":"at","refresh_token":"rt","account_id":"acc_1"}`
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	assert.Equal(t, "Bearer at", headers.Get("Authorization"))
	assert.Equal(t, "acc_1", headers.Get("chatgpt-account-id"))
}

func TestSetupRequestHeaderAgentKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyJSON := `{"type":"agent","agent_private_key":"` + base64.StdEncoding.EncodeToString(der) +
		`","agent_runtime_id":"rid","agent_task_id":"tid"}`
	c := testGinContext()
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelMeta.ApiKey = keyJSON
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err = adaptor.SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(headers.Get("Authorization"), "AgentAssertion "))
}

func TestSetupRequestHeaderRejectsPlainKey(t *testing.T) {
	c := testGinContext()
	info := testRelayInfo(relayconstant.RelayModeResponses)
	info.ChannelMeta.ApiKey = "sk-plain"
	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, info)
	require.Error(t, err)
}