package model

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ProxyProtocolHTTP    = "http"
	ProxyProtocolHTTPS   = "https"
	ProxyProtocolSOCKS5  = "socks5"
	ProxyProtocolSOCKS5H = "socks5h"
	ProxyProtocolSS      = "ss"
	ProxyStatusActive    = "active"
	ProxyStatusInactive  = "inactive"
)

type Proxy struct {
	ID       int    `json:"id"`
	Name     string `json:"name" gorm:"size:128;not null;index"`
	Protocol string `json:"protocol" gorm:"size:16;not null;index"`
	Host     string `json:"host" gorm:"size:255;not null"`
	Port     int    `json:"port" gorm:"not null"`

	UsernameEncrypted string `json:"-" gorm:"type:text"`
	PasswordEncrypted string `json:"-" gorm:"type:text"`
	IdentityHash      string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Status            string `json:"status" gorm:"size:16;not null;index"`

	LatencyMS      *int   `json:"latency_ms"`
	IPAddress      string `json:"ip_address,omitempty" gorm:"size:128"`
	Country        string `json:"country,omitempty" gorm:"size:128"`
	CountryCode    string `json:"country_code,omitempty" gorm:"size:16"`
	Region         string `json:"region,omitempty" gorm:"size:128"`
	City           string `json:"city,omitempty" gorm:"size:128"`
	QualityStatus  string `json:"quality_status,omitempty" gorm:"size:32"`
	QualityScore   *int   `json:"quality_score,omitempty"`
	QualityGrade   string `json:"quality_grade,omitempty" gorm:"size:4"`
	QualitySummary string `json:"quality_summary,omitempty" gorm:"type:text"`
	LastCheckedAt  *int64 `json:"last_checked_at,omitempty" gorm:"bigint"`
	LastHTTPStatus *int   `json:"last_http_status,omitempty"`
	LastError      string `json:"last_error,omitempty" gorm:"type:text"`

	CreatedTime int64          `json:"created_time" gorm:"bigint;not null"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint;not null"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	CredentialConfigured    bool  `json:"credential_configured" gorm:"-"`
	CredentialDecryptFailed bool  `json:"credential_decrypt_failed" gorm:"-"`
	BoundChannelCount       int64 `json:"bound_channel_count" gorm:"-"`
}

type ProxySummary struct {
	ID                      int    `json:"id"`
	Name                    string `json:"name"`
	Protocol                string `json:"protocol"`
	Host                    string `json:"host"`
	Port                    int    `json:"port"`
	Status                  string `json:"status"`
	LatencyMS               *int   `json:"latency_ms,omitempty"`
	IPAddress               string `json:"ip_address,omitempty"`
	Country                 string `json:"country,omitempty"`
	CountryCode             string `json:"country_code,omitempty"`
	Region                  string `json:"region,omitempty"`
	City                    string `json:"city,omitempty"`
	QualityStatus           string `json:"quality_status,omitempty"`
	QualityScore            *int   `json:"quality_score,omitempty"`
	QualityGrade            string `json:"quality_grade,omitempty"`
	QualitySummary          string `json:"quality_summary,omitempty"`
	LastCheckedAt           *int64 `json:"last_checked_at,omitempty"`
	CredentialConfigured    bool   `json:"credential_configured"`
	CredentialDecryptFailed bool   `json:"credential_decrypt_failed"`
	BoundChannelCount       int64  `json:"bound_channel_count"`
	LastHTTPStatus          *int   `json:"last_http_status,omitempty"`
	LastError               string `json:"last_error,omitempty"`
	QualityItems            []ProxyQualityItem `json:"quality_items,omitempty"`
}

// ProxyQualityItem is one AI target's latest quality check result, returned
// only in the quality-check API response (never persisted).
type ProxyQualityItem struct {
	Target     string `json:"target"`
	URL        string `json:"url,omitempty"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMS  int    `json:"latency_ms,omitempty"`
	Message    string `json:"message,omitempty"`
	CFRay      string `json:"cf_ray,omitempty"`
}

func (p *Proxy) BeforeCreate(tx *gorm.DB) error {
	if p.CreatedTime == 0 {
		p.CreatedTime = common.GetTimestamp()
	}
	if p.UpdatedTime == 0 {
		p.UpdatedTime = p.CreatedTime
	}
	if p.Status == "" {
		p.Status = ProxyStatusActive
	}
	if p.IdentityHash == "" && p.UsernameEncrypted == "" && p.PasswordEncrypted == "" {
		hash, err := common.ProxyIdentityHash(p.Protocol, p.Host, p.Port, "", "")
		if err != nil {
			return err
		}
		p.IdentityHash = hash
	}
	return p.Validate()
}

func (p *Proxy) Validate() error {
	if p == nil {
		return errors.New("proxy is nil")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("proxy name is required")
	}
	if !IsSupportedProxyProtocol(p.Protocol) {
		return fmt.Errorf("unsupported proxy protocol: %s", p.Protocol)
	}
	if strings.TrimSpace(p.Host) == "" {
		return errors.New("proxy host is required")
	}
	if strings.ContainsAny(p.Host, "/?#@") {
		return errors.New("proxy host contains invalid characters")
	}
	if p.Port < 1 || p.Port > 65535 {
		return errors.New("proxy port must be between 1 and 65535")
	}
	if p.Status != ProxyStatusActive && p.Status != ProxyStatusInactive {
		return fmt.Errorf("unsupported proxy status: %s", p.Status)
	}
	return nil
}

func IsSupportedProxyProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case ProxyProtocolHTTP, ProxyProtocolHTTPS, ProxyProtocolSOCKS5, ProxyProtocolSOCKS5H, ProxyProtocolSS:
		return true
	default:
		return false
	}
}

func ParseProxyEndpoint(raw string) (protocol, host string, port int, username, password string, err error) {
	parsed, err := common.ParseProxyURLStrict(raw)
	if err != nil {
		return "", "", 0, "", "", err
	}
	if parsed == nil {
		return "", "", 0, "", "", errors.New("proxy URL is required")
	}
	protocol = strings.ToLower(parsed.Scheme)
	host = parsed.Hostname()
	port = 0
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return "", "", 0, "", "", errors.New("proxy URL must include a valid port")
		}
	} else if protocol == ProxyProtocolSOCKS5 || protocol == ProxyProtocolSOCKS5H {
		port = 1080
	} else {
		return "", "", 0, "", "", errors.New("proxy URL must include a port")
	}
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	return protocol, host, port, username, password, nil
}

func (p *Proxy) URL(username, password string) string {
	if p == nil {
		return ""
	}
	u := &url.URL{Scheme: p.Protocol, Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port))}
	if username != "" || password != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String()
}

func (p *Proxy) IsActive() bool { return p != nil && p.Status == ProxyStatusActive }

func (p *Proxy) Summary() ProxySummary {
	if p == nil {
		return ProxySummary{}
	}
	return ProxySummary{
		ID: p.ID, Name: p.Name, Protocol: p.Protocol, Host: p.Host, Port: p.Port, Status: p.Status,
		LatencyMS: p.LatencyMS, IPAddress: p.IPAddress, Country: p.Country, CountryCode: p.CountryCode,
		Region: p.Region, City: p.City, QualityStatus: p.QualityStatus, QualityScore: p.QualityScore,
		QualityGrade: p.QualityGrade, QualitySummary: p.QualitySummary, LastCheckedAt: p.LastCheckedAt,
		CredentialConfigured:    p.UsernameEncrypted != "" || p.PasswordEncrypted != "",
		CredentialDecryptFailed: p.CredentialDecryptFailed, BoundChannelCount: p.BoundChannelCount,
		LastHTTPStatus: p.LastHTTPStatus, LastError: p.LastError,
	}
}

func (p *Proxy) Insert() error {
	return DB.Create(p).Error
}

func (p *Proxy) Update() error {
	p.UpdatedTime = time.Now().Unix()
	if err := p.Validate(); err != nil {
		return err
	}
	return DB.Model(&Proxy{}).Where("id = ?", p.ID).Select(
		"name", "protocol", "host", "port", "username_encrypted", "password_encrypted", "identity_hash", "status",
		"latency_ms", "ip_address", "country", "country_code", "region", "city", "quality_status",
		"quality_score", "quality_grade", "quality_summary", "last_checked_at", "last_http_status", "last_error", "updated_time",
	).Updates(p).Error
}

func (p *Proxy) Delete() error { return DB.Delete(p).Error }

func GetProxyByID(id int) (*Proxy, error) {
	var proxy Proxy
	if err := DB.First(&proxy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

func ListProxies(offset, limit int, status, search string) ([]*Proxy, int64, error) {
	query := DB.Model(&Proxy{})
	if status == ProxyStatusActive || status == ProxyStatusInactive {
		query = query.Where("status = ?", status)
	}
	if search = strings.TrimSpace(search); search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR host LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var proxies []*Proxy
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&proxies).Error; err != nil {
		return nil, 0, err
	}
	return proxies, total, nil
}

func ListActiveProxies() ([]*Proxy, error) {
	var proxies []*Proxy
	err := DB.Where("status = ?", ProxyStatusActive).Order("name ASC").Find(&proxies).Error
	return proxies, err
}
