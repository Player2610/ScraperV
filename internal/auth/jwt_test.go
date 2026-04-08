//go:build !integration

package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setJWTSecret(t *testing.T, secret string) {
	t.Helper()
	t.Setenv("JWT_SECRET", secret)
}

func TestIssueToken_ReturnsNonEmptyString(t *testing.T) {
	setJWTSecret(t, "test-secret-key")

	token, err := IssueToken(42, "customer")

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	// JWT has 3 dot-separated segments
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3, "JWT should have 3 dot-separated segments")
}

func TestIssueToken_NoSecret_ReturnsError(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	token, err := IssueToken(1, "customer")

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidateToken_ReturnsCorrectClaims(t *testing.T) {
	setJWTSecret(t, "test-secret-key")

	token, err := IssueToken(99, "operator")
	require.NoError(t, err)

	claims, err := ValidateToken(token)

	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, int64(99), claims.Sub)
	assert.Equal(t, "operator", claims.Role)
}

func TestValidateToken_TamperedToken_ReturnsError(t *testing.T) {
	setJWTSecret(t, "test-secret-key")

	token, err := IssueToken(1, "customer")
	require.NoError(t, err)

	// Tamper by replacing the signature segment
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	tampered := parts[0] + "." + parts[1] + ".invalidsignature"

	claims, err := ValidateToken(tampered)

	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestValidateToken_WrongSecret_ReturnsError(t *testing.T) {
	// Issue with one secret, validate with another
	t.Setenv("JWT_SECRET", "secret-a")
	token, err := IssueToken(1, "customer")
	require.NoError(t, err)

	t.Setenv("JWT_SECRET", "secret-b")
	claims, err := ValidateToken(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestValidateToken_ExpiredToken_ReturnsError(t *testing.T) {
	secret := "test-secret-key"
	setJWTSecret(t, secret)

	// Build a token that expired 1 hour ago
	expiredClaims := Claims{
		Sub:  7,
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)

	claims, err := ValidateToken(signed)

	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestValidateToken_NoSecret_ReturnsError(t *testing.T) {
	setJWTSecret(t, "test-secret-key")
	token, err := IssueToken(1, "customer")
	require.NoError(t, err)

	t.Setenv("JWT_SECRET", "")

	claims, err := ValidateToken(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}
