package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const proxySecretCiphertextVersion = "v1"

var errProxyStableSecretRequired = errors.New("stable CRYPTO_SECRET or SESSION_SECRET is required for proxy credentials")

func proxyStableSecret() ([]byte, error) {
	for _, name := range []string{"CRYPTO_SECRET", "SESSION_SECRET"} {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" || value == "random_string" {
			continue
		}
		digest := sha256.Sum256([]byte(value))
		return digest[:], nil
	}
	return nil, errProxyStableSecretRequired
}

func EncryptProxySecret(plaintext string) (string, error) {
	key, err := proxyStableSecret()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create proxy credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create proxy credential gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate proxy credential nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return proxySecretCiphertextVersion + ":" + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func DecryptProxySecret(encoded string) (string, error) {
	key, err := proxyStableSecret()
	if err != nil {
		return "", err
	}
	prefix := proxySecretCiphertextVersion + ":"
	if !strings.HasPrefix(encoded, prefix) {
		return "", errors.New("unsupported proxy credential ciphertext")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(encoded, prefix))
	if err != nil {
		return "", fmt.Errorf("decode proxy credential ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create proxy credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create proxy credential gcm: %w", err)
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("invalid proxy credential ciphertext")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("unable to decrypt proxy credential")
	}
	return string(plaintext), nil
}

func ProxyStableSecretConfigured() bool {
	_, err := proxyStableSecret()
	return err == nil
}

func ProxyIdentityHash(protocol, host string, port int, username, password string) (string, error) {
	payload := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%s\x00%s",
		strings.ToLower(strings.TrimSpace(protocol)),
		strings.ToLower(strings.TrimSpace(host)),
		port,
		username,
		password,
	)
	if username == "" && password == "" {
		digest := sha256.Sum256([]byte(payload))
		return base64.RawURLEncoding.EncodeToString(digest[:]), nil
	}
	key, err := proxyStableSecret()
	if err != nil {
		return "", err
	}
	return GenerateHMACWithKey(key, payload), nil
}
