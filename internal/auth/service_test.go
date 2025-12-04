package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	assert.NotNil(t, service)
	assert.Equal(t, []byte("test-secret"), service.jwtSecret)
	assert.Equal(t, 24*time.Hour, service.jwtExpiration)
	assert.Equal(t, 10, service.bcryptCost)
}

func TestNewService_DifferentConfigs(t *testing.T) {
	tests := []struct {
		name           string
		secret         string
		expirationHrs  int
		bcryptCost     int
		wantExpiration time.Duration
	}{
		{
			name:           "short expiration",
			secret:         "secret1",
			expirationHrs:  1,
			bcryptCost:     4,
			wantExpiration: 1 * time.Hour,
		},
		{
			name:           "long expiration",
			secret:         "longer-secret-key",
			expirationHrs:  168,
			bcryptCost:     12,
			wantExpiration: 168 * time.Hour,
		},
		{
			name:           "zero expiration",
			secret:         "zero",
			expirationHrs:  0,
			bcryptCost:     10,
			wantExpiration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.secret, tt.expirationHrs, tt.bcryptCost)
			assert.Equal(t, []byte(tt.secret), service.jwtSecret)
			assert.Equal(t, tt.wantExpiration, service.jwtExpiration)
			assert.Equal(t, tt.bcryptCost, service.bcryptCost)
		})
	}
}

func TestHashPassword(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	hash, err := service.HashPassword("testpassword123")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "testpassword123", hash)

	// Hash should be a bcrypt hash starting with $2a$ or $2b$
	assert.Contains(t, hash, "$2")
}

func TestHashPassword_DifferentPasswords(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	passwords := []string{
		"short",
		"medium-length-password",
		"very-long-password-with-special-characters!@#$%^&*()",
		"unicode-password-日本語",
		"",
	}

	for _, pwd := range passwords {
		t.Run(pwd, func(t *testing.T) {
			hash, err := service.HashPassword(pwd)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)
		})
	}
}

func TestHashPassword_SamePasswordDifferentHashes(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	hash1, err := service.HashPassword("samepassword")
	require.NoError(t, err)

	hash2, err := service.HashPassword("samepassword")
	require.NoError(t, err)

	// Due to salting, same password should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestVerifyPassword(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	password := "correctpassword"
	hash, err := service.HashPassword(password)
	require.NoError(t, err)

	// Correct password should verify
	err = service.VerifyPassword(hash, password)
	assert.NoError(t, err)
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	password := "correctpassword"
	hash, err := service.HashPassword(password)
	require.NoError(t, err)

	// Wrong password should fail
	err = service.VerifyPassword(hash, "wrongpassword")
	assert.Error(t, err)
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	// Invalid hash should fail
	err := service.VerifyPassword("not-a-valid-hash", "password")
	assert.Error(t, err)
}

func TestVerifyPassword_EmptyPassword(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	hash, err := service.HashPassword("")
	require.NoError(t, err)

	// Empty password should verify against its hash
	err = service.VerifyPassword(hash, "")
	assert.NoError(t, err)

	// Non-empty password should not verify against empty password hash
	err = service.VerifyPassword(hash, "something")
	assert.Error(t, err)
}

func TestGenerateToken(t *testing.T) {
	service := NewService("test-secret-key-for-jwt", 24, 10)

	userID := int64(123)
	username := "testuser"

	token, jti, expiresAt, err := service.GenerateToken(userID, username)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.NotEmpty(t, jti)
	assert.True(t, expiresAt.After(time.Now()))

	// Token should expire approximately 24 hours from now
	expectedExpiry := time.Now().Add(24 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, expiresAt, time.Minute)
}

func TestGenerateToken_UniqueJTI(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	_, jti1, _, err := service.GenerateToken(1, "user1")
	require.NoError(t, err)

	_, jti2, _, err := service.GenerateToken(1, "user1")
	require.NoError(t, err)

	// Each call should generate a unique JTI
	assert.NotEqual(t, jti1, jti2)
}

func TestGenerateToken_ValidTokenFormat(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	token, _, _, err := service.GenerateToken(1, "testuser")
	require.NoError(t, err)

	// JWT tokens have 3 parts separated by dots
	parts := splitDot(token)
	assert.Len(t, parts, 3)
}

func splitDot(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func TestValidateToken(t *testing.T) {
	service := NewService("test-secret-key", 24, 10)

	userID := int64(456)
	username := "admin"

	token, jti, _, err := service.GenerateToken(userID, username)
	require.NoError(t, err)

	claims, err := service.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, jti, claims.ID)
	assert.Equal(t, "terraform-mirror", claims.Issuer)
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	service1 := NewService("secret-1", 24, 10)
	service2 := NewService("secret-2", 24, 10)

	token, _, _, err := service1.GenerateToken(1, "user")
	require.NoError(t, err)

	// Token signed with different secret should fail
	_, err = service2.ValidateToken(token)
	assert.Error(t, err)
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	// Create service with 0 hour expiration
	service := NewService("test-secret", 0, 10)

	token, _, _, err := service.GenerateToken(1, "user")
	require.NoError(t, err)

	// Token with 0 expiration should be immediately expired
	_, err = service.ValidateToken(token)
	assert.Error(t, err)
}

func TestValidateToken_MalformedToken(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt-token"},
		{"partial", "eyJhbGciOiJIUzI1NiJ9"},
		{"wrong-format", "a.b"},
		{"four-parts", "a.b.c.d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ValidateToken(tt.token)
			assert.Error(t, err)
		})
	}
}

func TestValidateToken_WrongAlgorithm(t *testing.T) {
	service := NewService("test-secret", 24, 10)

	// Create a token with a different algorithm (none)
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		UserID:   1,
		Username: "hacker",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	// SigningMethodNone requires "none" as the key
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	// Should fail validation due to wrong algorithm
	_, err = service.ValidateToken(tokenString)
	assert.Error(t, err)
}

func TestGenerateRandomPassword(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   int
	}{
		{"minimum length", 8, 8},
		{"requested length", 16, 16},
		{"short request uses minimum", 4, 8},
		{"longer password", 32, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pwd, err := GenerateRandomPassword(tt.length)
			require.NoError(t, err)
			assert.Len(t, pwd, tt.want)
		})
	}
}

func TestGenerateRandomPassword_Uniqueness(t *testing.T) {
	passwords := make(map[string]bool)

	for i := 0; i < 100; i++ {
		pwd, err := GenerateRandomPassword(16)
		require.NoError(t, err)

		// Each password should be unique
		assert.False(t, passwords[pwd], "duplicate password generated")
		passwords[pwd] = true
	}
}

func TestGenerateRandomPassword_MinimumEnforced(t *testing.T) {
	// Even with 0 or negative length, should get minimum 8
	pwd, err := GenerateRandomPassword(0)
	require.NoError(t, err)
	assert.Len(t, pwd, 8)

	pwd, err = GenerateRandomPassword(-5)
	require.NoError(t, err)
	assert.Len(t, pwd, 8)
}

func TestIntegration_FullAuthFlow(t *testing.T) {
	service := NewService("integration-test-secret", 24, 10)

	// 1. Create user with password
	password := "user-password-123"
	hash, err := service.HashPassword(password)
	require.NoError(t, err)

	// 2. Verify password
	err = service.VerifyPassword(hash, password)
	require.NoError(t, err)

	// 3. Generate token
	userID := int64(999)
	username := "integration-user"
	token, _, _, err := service.GenerateToken(userID, username)
	require.NoError(t, err)

	// 4. Validate token
	claims, err := service.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
}
