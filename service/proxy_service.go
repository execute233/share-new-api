package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var ErrProxyNotFound = errors.New("proxy not found")

// ResolvedProxy is the runtime-only proxy decision for a channel. URL is empty for direct routing.
type ResolvedProxy struct {
	ProxyID *int
	URL     string
}

func ResolveChannelProxy(channel *model.Channel) (ResolvedProxy, error) {
	if channel == nil || channel.ProxyID == nil {
		return ResolvedProxy{}, nil
	}
	proxy, err := model.GetProxyByID(*channel.ProxyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolvedProxy{}, nil
		}
		return ResolvedProxy{}, err
	}
	if !proxy.IsActive() {
		return ResolvedProxy{ProxyID: channel.ProxyID}, nil
	}
	username, password, err := decryptProxyCredentials(proxy)
	if err != nil {
		return ResolvedProxy{}, err
	}
	return ResolvedProxy{ProxyID: channel.ProxyID, URL: proxy.URL(username, password)}, nil
}

func ProxyCredentials(proxy *model.Proxy) (string, string, error) {
	return decryptProxyCredentials(proxy)
}

func decryptProxyCredentials(proxy *model.Proxy) (string, string, error) {
	if proxy == nil {
		return "", "", ErrProxyNotFound
	}
	var username, password string
	var err error
	if proxy.UsernameEncrypted != "" {
		username, err = common.DecryptProxySecret(proxy.UsernameEncrypted)
		if err != nil {
			return "", "", fmt.Errorf("decrypt proxy username: %w", err)
		}
	}
	if proxy.PasswordEncrypted != "" {
		password, err = common.DecryptProxySecret(proxy.PasswordEncrypted)
		if err != nil {
			return "", "", fmt.Errorf("decrypt proxy password: %w", err)
		}
	}
	return username, password, nil
}

func EncryptProxyCredentials(username, password string) (encryptedUsername, encryptedPassword string, err error) {
	if strings.TrimSpace(username) == "" && strings.TrimSpace(password) == "" {
		return "", "", nil
	}
	if username != "" {
		encryptedUsername, err = common.EncryptProxySecret(username)
		if err != nil {
			return "", "", err
		}
	}
	if password != "" {
		encryptedPassword, err = common.EncryptProxySecret(password)
		if err != nil {
			return "", "", err
		}
	}
	return encryptedUsername, encryptedPassword, nil
}

func ValidateProxyBinding(proxyID *int) error {
	if proxyID == nil {
		return nil
	}
	if *proxyID <= 0 {
		return errors.New("proxy_id must be positive")
	}
	if _, err := model.GetProxyByID(*proxyID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProxyNotFound
		}
		return err
	}
	return nil
}

func DeleteProxyAndClearChannels(ctx context.Context, proxyID int) (int64, error) {
	if proxyID <= 0 {
		return 0, errors.New("invalid proxy id")
	}
	tx := model.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	var bound int64
	if err := tx.Model(&model.Channel{}).Where("proxy_id = ?", proxyID).Count(&bound).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Model(&model.Channel{}).Where("proxy_id = ?", proxyID).Update("proxy_id", nil).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Delete(&model.Proxy{}, proxyID).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return bound, nil
}
