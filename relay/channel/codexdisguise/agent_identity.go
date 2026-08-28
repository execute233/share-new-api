package codexdisguise

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

type agentIdentityKey struct {
	runtimeID  string
	privateKey ed25519.PrivateKey
	taskID     string
}

// parseAgentIdentityPrivateKey 解析 base64 编码的 PKCS#8 Ed25519 私钥。
func parseAgentIdentityPrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw := strings.TrimSpace(encoded)
	if raw == "" {
		return nil, errors.New("agent identity private key is missing")
	}
	der, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("agent identity private key is not valid base64")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, errors.New("agent identity private key is not valid PKCS#8")
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("agent identity private key is not Ed25519")
	}
	return privateKey, nil
}

func agentIdentityKeyFromDisguiseKey(k *DisguiseKey) (agentIdentityKey, error) {
	if k == nil || k.Type != DisguiseKeyTypeAgent {
		return agentIdentityKey{}, errors.New("agent identity key requires agent type key")
	}
	privateKey, err := parseAgentIdentityPrivateKey(k.AgentPrivateKey)
	if err != nil {
		return agentIdentityKey{}, err
	}
	if k.AgentRuntimeID == "" {
		return agentIdentityKey{}, errors.New("agent identity runtime id is missing")
	}
	return agentIdentityKey{
		runtimeID:  k.AgentRuntimeID,
		privateKey: privateKey,
		taskID:     k.AgentTaskID,
	}, nil
}

// buildAgentAssertion 构造 AgentAssertion 认证头：ed25519 签名 runtimeID:taskID:timestamp。
func buildAgentAssertion(key agentIdentityKey, now time.Time) (string, error) {
	if key.runtimeID == "" || key.taskID == "" {
		return "", errors.New("agent identity runtime or task id is missing")
	}
	timestamp := now.UTC().Format(time.RFC3339)
	payload := []byte(key.runtimeID + ":" + key.taskID + ":" + timestamp)
	signature, err := key.privateKey.Sign(nil, payload, crypto.Hash(0))
	if err != nil {
		return "", errors.New("failed to sign agent assertion")
	}
	envelope := map[string]string{
		"agent_runtime_id": key.runtimeID,
		"task_id":          key.taskID,
		"timestamp":        timestamp,
		"signature":        base64.StdEncoding.EncodeToString(signature),
	}
	encoded, err := common.Marshal(envelope)
	if err != nil {
		return "", errors.New("failed to serialize agent assertion")
	}
	return "AgentAssertion " + base64.RawURLEncoding.EncodeToString(encoded), nil
}

// signAgentTaskRegistration 构造 task 注册签名：ed25519 签名 runtimeID:timestamp。
func signAgentTaskRegistration(key agentIdentityKey, timestamp time.Time) (string, string, error) {
	if key.runtimeID == "" {
		return "", "", errors.New("agent identity runtime id is missing")
	}
	formatted := timestamp.UTC().Format(time.RFC3339)
	signature, err := key.privateKey.Sign(nil, []byte(key.runtimeID+":"+formatted), crypto.Hash(0))
	if err != nil {
		return "", "", errors.New("failed to sign agent task registration")
	}
	return formatted, base64.StdEncoding.EncodeToString(signature), nil
}

// decryptAgentTaskID 用派生 X25519 私钥解密 encrypted_task_id。
func decryptAgentTaskID(key agentIdentityKey, encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", errors.New("encrypted agent task id is not valid base64")
	}
	seed := key.privateKey.Seed()
	digest := sha512.Sum512(seed)
	var curvePrivate [32]byte
	copy(curvePrivate[:], digest[:32])
	curvePrivate[0] &= 248
	curvePrivate[31] &= 127
	curvePrivate[31] |= 64
	curvePublicBytes, err := curve25519.X25519(curvePrivate[:], curve25519.Basepoint)
	if err != nil {
		return "", errors.New("failed to derive agent identity decryption key")
	}
	var curvePublic [32]byte
	copy(curvePublic[:], curvePublicBytes)
	plaintext, ok := box.OpenAnonymous(nil, ciphertext, &curvePublic, &curvePrivate)
	if !ok {
		return "", errors.New("failed to decrypt encrypted agent task id")
	}
	taskID := strings.TrimSpace(string(plaintext))
	if taskID == "" {
		return "", errors.New("decrypted agent task id is empty")
	}
	return taskID, nil
}