package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const proxyProbeMaxBodyBytes = 256 * 1024

type ProxyProbeResult struct {
	LatencyMS     int
	HTTPStatus    int
	IPAddress     string
	Country       string
	CountryCode   string
	Region        string
	City          string
	QualityStatus string
	QualityScore  int
	QualityGrade  string
	Summary       string
	Error         string
}

type ProxyProbe interface {
	Probe(context.Context, ResolvedProxy, bool) ProxyProbeResult
}

type DefaultProxyProbe struct{}

type proxyQualityTarget struct {
	name    string
	url     string
	allowed map[int]struct{}
}

var defaultProxyQualityTargets = []proxyQualityTarget{
	{
		name:    "openai",
		url:     "https://api.openai.com/v1/models",
		allowed: map[int]struct{}{http.StatusUnauthorized: {}},
	},
	{
		name: "anthropic",
		url:  "https://api.anthropic.com/v1/messages",
		allowed: map[int]struct{}{
			http.StatusUnauthorized:     {},
			http.StatusMethodNotAllowed: {},
			http.StatusNotFound:         {},
			http.StatusBadRequest:       {},
		},
	},
	{
		name:    "gemini",
		url:     "https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta",
		allowed: map[int]struct{}{http.StatusOK: {}},
	},
	{
		name:    "grok",
		url:     "https://api.x.ai/v1/models",
		allowed: map[int]struct{}{http.StatusUnauthorized: {}},
	},
}

func (DefaultProxyProbe) Probe(ctx context.Context, resolved ResolvedProxy, quality bool) ProxyProbeResult {
	client, err := GetHttpClientWithProxy(resolved.URL)
	if err != nil {
		return proxyProbeFailure("failed to create proxy client")
	}
	result := probeExitInfo(ctx, client)
	if result.Error != "" || !quality {
		if result.Error == "" {
			result.QualityStatus, result.QualityScore, result.QualityGrade = "healthy", 100, "A"
			result.Summary = "代理出口连通正常"
		}
		return result
	}

	passed, warned, failed, challenged := 1, 0, 0, 0
	for _, target := range defaultProxyQualityTargets {
		status := probeQualityTarget(ctx, client, target)
		switch status {
		case "pass":
			passed++
		case "warn":
			warned++
		case "challenge":
			challenged++
		default:
			failed++
		}
	}
	score := 100 - warned*10 - failed*22 - challenged*30
	if score < 0 {
		score = 0
	}
	result.QualityScore = score
	result.QualityGrade = proxyQualityGrade(score)
	result.Summary = fmt.Sprintf("通过 %d 项，告警 %d 项，失败 %d 项，挑战 %d 项", passed, warned, failed, challenged)
	switch {
	case challenged > 0:
		result.QualityStatus = "challenge"
	case failed > 0:
		result.QualityStatus = "failed"
	case warned > 0:
		result.QualityStatus = "warn"
	default:
		result.QualityStatus = "healthy"
	}
	return result
}

func probeExitInfo(ctx context.Context, client *http.Client) ProxyProbeResult {
	type probeTarget struct{ url, parser string }
	var last ProxyProbeResult
	for _, target := range []probeTarget{{"http://ip-api.com/json/?lang=zh-CN", "ip-api"}, {"https://api64.ipify.org?format=json", "ipify"}} {
		started := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.url, nil)
		if err != nil {
			return proxyProbeFailure("failed to create probe request")
		}
		resp, err := client.Do(req)
		if err != nil {
			last = proxyProbeFailure("proxy connection failed")
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, proxyProbeMaxBodyBytes+1))
		_ = resp.Body.Close()
		last.LatencyMS = int(time.Since(started).Milliseconds())
		last.HTTPStatus = resp.StatusCode
		if readErr != nil || len(body) > proxyProbeMaxBodyBytes || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			last.Error = "proxy probe failed"
			continue
		}
		if target.parser == "ip-api" {
			var payload struct{ Status, Query, City, RegionName, Country, CountryCode string }
			if err := common.Unmarshal(body, &payload); err == nil && strings.EqualFold(payload.Status, "success") && payload.Query != "" {
				last.IPAddress, last.City, last.Region, last.Country, last.CountryCode = payload.Query, payload.City, payload.RegionName, payload.Country, payload.CountryCode
				last.Error = ""
				return last
			}
		} else {
			var payload struct {
				IP string `json:"ip"`
			}
			if err := common.Unmarshal(body, &payload); err == nil && payload.IP != "" {
				last.IPAddress = payload.IP
				last.Error = ""
				return last
			}
		}
		last.Error = "proxy probe response invalid"
	}
	if last.Error == "" {
		last.Error = "proxy connection failed"
	}
	last.QualityStatus, last.QualityScore, last.QualityGrade = "failed", 78, "B"
	return last
}

func probeQualityTarget(ctx context.Context, client *http.Client, target proxyQualityTarget) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.url, nil)
	if err != nil {
		return "fail"
	}
	req.Header.Set("Accept", "application/json,text/html,*/*")
	req.Header.Set("User-Agent", "new-api-proxy-quality/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "fail"
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
	lowerBody := strings.ToLower(string(body))
	if resp.StatusCode == http.StatusForbidden && (resp.Header.Get("CF-Ray") != "" || strings.Contains(lowerBody, "cloudflare") || strings.Contains(lowerBody, "challenge")) {
		return "challenge"
	}
	if _, ok := target.allowed[resp.StatusCode]; ok {
		return "pass"
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "pass"
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "warn"
	}
	return "fail"
}

func proxyProbeFailure(message string) ProxyProbeResult {
	return ProxyProbeResult{QualityStatus: "failed", QualityScore: 78, QualityGrade: "B", Error: message}
}

func proxyQualityGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}
