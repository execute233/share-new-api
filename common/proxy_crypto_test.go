package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxySecretRoundTripUsesExplicitStableSecret(t *testing.T) {
	t.Setenv("CRYPTO_SECRET", "proxy-test-secret")
	t.Setenv("SESSION_SECRET", "")

	ciphertext, err := EncryptProxySecret("proxy-password")
	require.NoError(t, err)
	require.NotEqual(t, "proxy-password", ciphertext)

	plaintext, err := DecryptProxySecret(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "proxy-password", plaintext)
}

func TestProxySecretRejectsMissingExplicitSecret(t *testing.T) {
	previousCrypto, cryptoSet := os.LookupEnv("CRYPTO_SECRET")
	previousSession, sessionSet := os.LookupEnv("SESSION_SECRET")
	t.Cleanup(func() {
		if cryptoSet {
			_ = os.Setenv("CRYPTO_SECRET", previousCrypto)
		} else {
			_ = os.Unsetenv("CRYPTO_SECRET")
		}
		if sessionSet {
			_ = os.Setenv("SESSION_SECRET", previousSession)
		} else {
			_ = os.Unsetenv("SESSION_SECRET")
		}
	})
	_ = os.Unsetenv("CRYPTO_SECRET")
	_ = os.Unsetenv("SESSION_SECRET")

	_, err := EncryptProxySecret("proxy-password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stable")
}

func TestProxySecretRejectsKnownPlaceholder(t *testing.T) {
	t.Setenv("CRYPTO_SECRET", "random_string")
	t.Setenv("SESSION_SECRET", "")

	_, err := EncryptProxySecret("proxy-password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stable")
}
