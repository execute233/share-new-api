package codexdisguise

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

const (
	codexFingerprintIDsContextKey = "codex_disguise_fingerprint_ids"
	codexThreadIDContextKey       = "codex_disguise_thread_id"
)

// turnStates 全局 turn-state 溯源表：多下游共享统一会话时，剥离异 thread 回带的 blob。
var turnStates = newTurnStateRegistry(time.Hour)

// nowFunc 可注入的时钟（测试用），生产环境为 time.Now。
var nowFunc = time.Now

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("codex disguise channel: endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("codex disguise channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("codex disguise channel: endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("codex disguise channel: /v1/chat/completions endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("codex disguise channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	var path string
	switch info.RelayMode {
	case relayconstant.RelayModeResponses:
		path = "/backend-api/codex/responses"
	case relayconstant.RelayModeResponsesCompact:
		path = "/backend-api/codex/responses/compact"
	case relayconstant.RelayModeAlphaSearch:
		path = "/backend-api/codex/alpha/search"
	default:
		return "", errors.New("codex disguise channel: only /v1/responses, /v1/responses/compact and /v1/alpha/search are supported")
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info != nil && info.ChannelSetting.SystemPrompt != "" {
		systemPrompt := info.ChannelSetting.SystemPrompt
		if len(request.Instructions) == 0 {
			if b, err := common.Marshal(systemPrompt); err == nil {
				request.Instructions = b
			} else {
				return nil, err
			}
		} else if info.ChannelSetting.SystemPromptOverride {
			var existing string
			if err := common.Unmarshal(request.Instructions, &existing); err == nil {
				existing = strings.TrimSpace(existing)
				if existing == "" {
					if b, err := common.Marshal(systemPrompt); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				} else {
					if b, err := common.Marshal(systemPrompt + "\n" + existing); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				}
			} else {
				if b, err := common.Marshal(systemPrompt); err == nil {
					request.Instructions = b
				} else {
					return nil, err
				}
			}
		}
	}
	if len(request.Instructions) == 0 {
		request.Instructions = []byte(`""`)
	}

	if info != nil && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		request.Store = []byte("false")
		request.MaxOutputTokens = nil
		request.Temperature = nil
		request.FrequencyPenalty = nil
		request.PresencePenalty = nil
	}

	a.stageCodexFingerprintIDs(c, info)
	return request, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key, err := ParseDisguiseKey(info.ApiKey)
	if err != nil {
		return err
	}

	switch key.Type {
	case DisguiseKeyTypeSub2API:
		req.Set("Authorization", "Bearer "+key.APIKey)
	case DisguiseKeyTypeOAuth:
		if key.AccessToken == "" {
			return errors.New("codex disguise channel: access_token is required (refresh not supported on hot path)")
		}
		req.Set("Authorization", "Bearer "+key.AccessToken)
		if key.AccountID != "" {
			req.Set("chatgpt-account-id", key.AccountID)
		}
	case DisguiseKeyTypeAgent:
		agentKey, err := agentIdentityKeyFromDisguiseKey(key)
		if err != nil {
			return err
		}
		assertion, err := buildAgentAssertion(agentKey, nowFunc())
		if err != nil {
			return err
		}
		req.Set("Authorization", assertion)
	}

	canonicalUA := buildCodexCLIUserAgent(codexClientVersionFromSettings(info))
	version := codexClientVersionFromSettings(info)
	enforce := codexDisguiseEnforceIdentity(info)

	ensureCodexIdentityHeaders(req, canonicalUA, version)
	a.applyFingerprintHeaders(c, info, req)
	a.guardTurnState(c, info, req)
	enforceCodexIdentityHeaders(req, canonicalUA, version, enforce)

	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	a.captureTurnState(c, info, resp)
	switch info.RelayMode {
	case relayconstant.RelayModeResponsesCompact:
		return openai.OaiResponsesCompactionHandler(c, resp)
	case relayconstant.RelayModeResponses:
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)
	default:
		return nil, types.NewError(errors.New("codex disguise channel: endpoint not supported"), types.ErrorCodeInvalidRequest)
	}
}

// ---- 指纹 / turn-state 辅助（gin context 暂存共享 IDs）----

func (a *Adaptor) stageCodexFingerprintIDs(c *gin.Context, info *relaycommon.RelayInfo) {
	if c == nil || info == nil {
		return
	}
	mode := codexDisguiseFingerprintMode(info)
	seed := codexDisguiseFingerprintSeed(info)
	clientSessionID := ""
	if c.Request != nil {
		clientSessionID = strings.TrimSpace(c.Request.Header.Get("session-id"))
		if clientSessionID == "" {
			clientSessionID = strings.TrimSpace(c.Request.Header.Get("session_id"))
		}
	}
	ids := resolveCodexFingerprintIDs(mode, seed, clientSessionID)
	c.Set(codexFingerprintIDsContextKey, ids)
	if ids != nil {
		c.Set(codexThreadIDContextKey, ids.ThreadID)
	}
}

func (a *Adaptor) stagedCodexFingerprintIDs(c *gin.Context) *codexFingerprintIDs {
	if c == nil {
		return nil
	}
	value, ok := c.Get(codexFingerprintIDsContextKey)
	if !ok {
		return nil
	}
	ids, _ := value.(*codexFingerprintIDs)
	return ids
}

func (a *Adaptor) stagedThreadID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, _ := c.Get(codexThreadIDContextKey)
	threadID, _ := value.(string)
	return threadID
}

func (a *Adaptor) applyFingerprintHeaders(c *gin.Context, info *relaycommon.RelayInfo, req *http.Header) {
	if c == nil || req == nil {
		return
	}
	applyCodexFingerprintHeaders(req, a.stagedCodexFingerprintIDs(c))
}

func (a *Adaptor) guardTurnState(c *gin.Context, info *relaycommon.RelayInfo, req *http.Header) {
	if c == nil || req == nil || c.Request == nil {
		return
	}
	blob := strings.TrimSpace(c.Request.Header.Get("x-codex-turn-state"))
	if blob == "" {
		return
	}
	threadID := a.stagedThreadID(c)
	guarded := turnStates.guard(threadID, blob)
	if guarded == "" {
		req.Del("x-codex-turn-state")
		return
	}
	req.Set("x-codex-turn-state", guarded)
}

func (a *Adaptor) captureTurnState(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) {
	if c == nil || resp == nil {
		return
	}
	blob := strings.TrimSpace(resp.Header.Get("x-codex-turn-state"))
	if blob == "" {
		return
	}
	threadID := a.stagedThreadID(c)
	turnStates.note(threadID, blob)
	c.Header("x-codex-turn-state", blob)
}

// ---- 渠道配置读取（ChannelOtherSettings，relaykit/dto 新增字段）----

func codexDisguiseFingerprintMode(info *relaycommon.RelayInfo) FingerprintMode {
	if info != nil && info.ChannelOtherSettings.FingerprintMode != "" {
		return ParseFingerprintMode(info.ChannelOtherSettings.FingerprintMode)
	}
	return FingerprintModeSession
}

func codexDisguiseFingerprintSeed(info *relaycommon.RelayInfo) string {
	if info != nil {
		return strings.TrimSpace(info.ChannelOtherSettings.FingerprintSeed)
	}
	return ""
}

func codexClientVersionFromSettings(info *relaycommon.RelayInfo) string {
	if info != nil {
		if v := NormalizeCodexClientVersion(info.ChannelOtherSettings.CodexClientVersion); v != "" {
			return v
		}
	}
	return codexCLIVersion
}

func codexDisguiseEnforceIdentity(info *relaycommon.RelayInfo) bool {
	if info != nil && info.ChannelOtherSettings.EnforceIdentity != nil {
		return *info.ChannelOtherSettings.EnforceIdentity
	}
	return true
}