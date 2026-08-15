package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExternalIdentityClaimEnforcesSingleOwnerAtomically(t *testing.T) {
	truncateTables(t)

	first := User{Username: "claim-owner-one", Password: "password", AffCode: "claim-owner-one"}
	second := User{Username: "claim-owner-two", Password: "password", AffCode: "claim-owner-two"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, "oidc", "subject-123", first.Id)
	}))
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, "oidc", "subject-123", second.Id)
	})
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, "oidc", "subject-456", first.Id)
	})
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	var claims []ExternalIdentityClaim
	require.NoError(t, DB.Find(&claims).Error)
	require.Len(t, claims, 1)
	assert.Equal(t, first.Id, claims[0].UserId)
	assert.Equal(t, "subject-123", claims[0].Subject)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ReleaseExternalIdentityWithTx(tx, "oidc", first.Id)
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, "oidc", "subject-123", second.Id)
	}))
}
