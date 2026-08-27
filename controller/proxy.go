package controller

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type proxyWriteRequest struct {
	Name             string `json:"name"`
	Protocol         string `json:"protocol"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Status           string `json:"status"`
	ClearCredentials bool   `json:"clear_credentials"`
}

type quickAddProxyRequest struct {
	URLs []string `json:"urls"`
}

type quickAddProxyResult struct {
	Line    int                 `json:"line"`
	Created bool                `json:"created"`
	Skipped bool                `json:"skipped"`
	Proxy   *model.ProxySummary `json:"proxy,omitempty"`
	Error   string              `json:"error,omitempty"`
}

func proxySummary(proxy *model.Proxy) model.ProxySummary {
	if proxy != nil && (proxy.UsernameEncrypted != "" || proxy.PasswordEncrypted != "") {
		if _, _, err := service.ProxyCredentials(proxy); err != nil {
			proxy.CredentialDecryptFailed = true
		}
	}
	summary := proxy.Summary()
	if proxy != nil && proxy.ID > 0 {
		_ = model.DB.Model(&model.Channel{}).Where("proxy_id = ?", proxy.ID).Count(&summary.BoundChannelCount).Error
	}
	return summary
}

func validateAndEncryptProxy(req proxyWriteRequest, existing *model.Proxy) (*model.Proxy, error) {
	proxy := &model.Proxy{
		Name:     strings.TrimSpace(req.Name),
		Protocol: strings.ToLower(strings.TrimSpace(req.Protocol)),
		Host:     strings.TrimSpace(req.Host),
		Port:     req.Port,
		Status:   strings.ToLower(strings.TrimSpace(req.Status)),
	}
	if existing != nil {
		proxy.ID = existing.ID
		proxy.UsernameEncrypted = existing.UsernameEncrypted
		proxy.PasswordEncrypted = existing.PasswordEncrypted
		proxy.CreatedTime = existing.CreatedTime
	}
	if proxy.Status == "" {
		proxy.Status = model.ProxyStatusActive
	}
	identityUsername, identityPassword := "", ""
	if req.ClearCredentials {
		proxy.UsernameEncrypted = ""
		proxy.PasswordEncrypted = ""
	} else if req.Username != "" || req.Password != "" {
		username, password, err := service.EncryptProxyCredentials(req.Username, req.Password)
		if err != nil {
			return nil, err
		}
		proxy.UsernameEncrypted = username
		proxy.PasswordEncrypted = password
		identityUsername, identityPassword = req.Username, req.Password
	} else if existing != nil {
		var err error
		identityUsername, identityPassword, err = service.ProxyCredentials(existing)
		if err != nil {
			return nil, err
		}
	}
	identityHash, err := common.ProxyIdentityHash(proxy.Protocol, proxy.Host, proxy.Port, identityUsername, identityPassword)
	if err != nil {
		return nil, err
	}
	proxy.IdentityHash = identityHash
	if err := proxy.Validate(); err != nil {
		return nil, err
	}
	return proxy, nil
}

func ListProxies(c *gin.Context) {
	page := common.GetPageQuery(c)
	proxies, total, err := model.ListProxies(page.GetStartIdx(), page.GetPageSize(), c.Query("status"), c.Query("search"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]model.ProxySummary, 0, len(proxies))
	for _, proxy := range proxies {
		items = append(items, proxySummary(proxy))
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total, "page": page.GetPage(), "page_size": page.GetPageSize()})
}

func ListActiveProxies(c *gin.Context) {
	proxies, err := model.ListActiveProxies()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]model.ProxySummary, 0, len(proxies))
	for _, proxy := range proxies {
		items = append(items, proxySummary(proxy))
	}
	common.ApiSuccess(c, items)
}

func GetProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	proxy, err := model.GetProxyByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, proxySummary(proxy))
}

func CreateProxy(c *gin.Context) {
	var req proxyWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	proxy, err := validateAndEncryptProxy(req, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := proxy.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "proxy.create", map[string]interface{}{"id": proxy.ID, "name": proxy.Name})
	common.ApiSuccess(c, proxySummary(proxy))
}

func UpdateProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	var req proxyWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetProxyByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	proxy, err := validateAndEncryptProxy(req, existing)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	oldURL, oldURLerr := service.ResolveChannelProxy(&model.Channel{ProxyID: &id})
	if err := proxy.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	if oldURLerr == nil && oldURL.URL != "" {
		service.InvalidateProxyClient(oldURL.URL)
	}
	model.InitChannelCache()
	recordManageAudit(c, "proxy.update", map[string]interface{}{"id": proxy.ID, "name": proxy.Name})
	common.ApiSuccess(c, proxySummary(proxy))
}

func DeleteProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	proxy, err := model.GetProxyByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resolved, _ := service.ResolveChannelProxy(&model.Channel{ProxyID: &id})
	bound, err := service.DeleteProxyAndClearChannels(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if resolved.URL != "" {
		service.InvalidateProxyClient(resolved.URL)
	}
	model.InitChannelCache()
	recordManageAudit(c, "proxy.delete", map[string]interface{}{"id": id, "name": proxy.Name, "bound_channel_count": bound})
	common.ApiSuccess(c, gin.H{"bound_channel_count": bound})
}

func QuickAddProxies(c *gin.Context) {
	var req quickAddProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.URLs) == 0 {
		common.ApiErrorMsg(c, "urls is required")
		return
	}
	results := make([]quickAddProxyResult, 0, len(req.URLs))
	for index, rawURL := range req.URLs {
		protocol, host, port, username, password, err := model.ParseProxyEndpoint(rawURL)
		if err != nil {
			results = append(results, quickAddProxyResult{Line: index + 1, Error: sanitizeProxyInputError(err)})
			continue
		}
		identityHash, err := common.ProxyIdentityHash(protocol, host, port, username, password)
		if err != nil {
			results = append(results, quickAddProxyResult{Line: index + 1, Error: sanitizeProxyInputError(err)})
			continue
		}
		var count int64
		if err := model.DB.Model(&model.Proxy{}).Where("identity_hash = ?", identityHash).Count(&count).Error; err != nil {
			results = append(results, quickAddProxyResult{Line: index + 1, Error: "failed to check proxy"})
			continue
		}
		if count > 0 {
			results = append(results, quickAddProxyResult{Line: index + 1, Skipped: true})
			continue
		}
		usernameEncrypted, passwordEncrypted, err := service.EncryptProxyCredentials(username, password)
		if err != nil {
			results = append(results, quickAddProxyResult{Line: index + 1, Error: sanitizeProxyInputError(err)})
			continue
		}
		proxy := &model.Proxy{
			Name:              host,
			Protocol:          protocol,
			Host:              host,
			Port:              port,
			UsernameEncrypted: usernameEncrypted,
			PasswordEncrypted: passwordEncrypted,
			IdentityHash:      identityHash,
			Status:            model.ProxyStatusActive,
		}
		if err := proxy.Insert(); err != nil {
			results = append(results, quickAddProxyResult{Line: index + 1, Error: "failed to create proxy"})
			continue
		}
		summary := proxySummary(proxy)
		results = append(results, quickAddProxyResult{Line: index + 1, Created: true, Proxy: &summary})
	}
	recordManageAudit(c, "proxy.quick_add", map[string]interface{}{"count": len(req.URLs)})
	common.ApiSuccess(c, gin.H{"items": results})
}

func sanitizeProxyInputError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(message, "stable") || strings.HasPrefix(message, "proxy URL") || strings.Contains(message, "shadowsocks") {
		return message
	}
	return "invalid proxy configuration"
}

var proxyProbe service.ProxyProbe = service.DefaultProxyProbe{}

func TestProxy(c *gin.Context)         { runProxyProbe(c, false) }
func CheckProxyQuality(c *gin.Context) { runProxyProbe(c, true) }

func runProxyProbe(c *gin.Context, quality bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	proxy, err := model.GetProxyByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	username, password, err := service.ProxyCredentials(proxy)
	if err != nil {
		common.ApiErrorMsg(c, "proxy credentials are unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result := proxyProbe.Probe(ctx, service.ResolvedProxy{ProxyID: &id, URL: proxy.URL(username, password)}, quality)
	now := common.GetTimestamp()
	proxy.LastCheckedAt = &now
	proxy.LastHTTPStatus = &result.HTTPStatus
	proxy.LastError = result.Error
	proxy.LatencyMS = &result.LatencyMS
	proxy.IPAddress = result.IPAddress
	proxy.Country, proxy.CountryCode, proxy.Region, proxy.City = result.Country, result.CountryCode, result.Region, result.City
	proxy.QualityStatus = result.QualityStatus
	proxy.QualityScore = &result.QualityScore
	proxy.QualityGrade, proxy.QualitySummary = result.QualityGrade, result.Summary
	if err := proxy.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "proxy.quality_check", map[string]interface{}{"id": id, "quality": quality})
	common.ApiSuccess(c, proxySummary(proxy))
}
