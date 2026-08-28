package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// CodexDisguiseClientVersionOptionKey 管理员手配的 Codex 伪装出站版本。
// 空值 = 使用 GitHub 自动同步的最新稳定版。
const CodexDisguiseClientVersionOptionKey = "CodexDisguiseClientVersion"

var codexDisguiseVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}(-[0-9A-Za-z.]+)?$`)

// GetCodexDisguiseClientVersion 返回生效的 Codex 伪装客户端版本：
// 管理员手配 Option 优先，空则拉取 GitHub 最新稳定版（1h 缓存）。
func GetCodexDisguiseClientVersion(ctx context.Context, proxyURL string) (string, error) {
	if v := strings.TrimSpace(common.OptionMap[CodexDisguiseClientVersionOptionKey]); v != "" {
		if codexDisguiseVersionPattern.MatchString(v) {
			return v, nil
		}
		return "", errors.New("codex disguise client version option is invalid")
	}
	client, err := GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return "", err
	}
	return GetLatestCodexClientVersion(ctx, client)
}

// SetCodexDisguiseClientVersion 设置/清空管理员手配版本（空串清空）。
func SetCodexDisguiseClientVersion(version string) error {
	version = strings.TrimSpace(version)
	if version != "" && !codexDisguiseVersionPattern.MatchString(version) {
		return errors.New("codex disguise client version must match X.Y.Z or X.Y.Z-alpha.N")
	}
	if err := model.UpdateOption(CodexDisguiseClientVersionOptionKey, version); err != nil {
		return err
	}
	model.InitOptionMap()
	return nil
}