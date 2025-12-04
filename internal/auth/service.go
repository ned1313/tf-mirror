package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service handles authentication operations
type Service struct {
	jwtSecret     []byte
	jwtExpiration time.Duration
	bcryptCost    int
}

// Claims represents JWT claims for admin users
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewService creates a new authentication service
func NewService(jwtSecret string, jwtExpirationHours int, bcryptCost int) *Service {
	return &Service{
		jwtSecret:     []byte(jwtSecret),
		jwtExpiration: time.Duration(jwtExpirationHours) * time.Hour,
		bcryptCost:    bcryptCost,
	}
}

// HashPassword hashes a password using bcrypt
func (s *Service) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks if a password matches the hash
func (s *Service) VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateToken creates a new JWT token for a user
func (s *Service) GenerateToken(userID int64, username string) (string, string, time.Time, error) {
	// Generate a random JTI (JWT ID) for session tracking
	jtiBytes := make([]byte, 32)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to generate JTI: %w", err)
	}
	jti := base64.URLEncoding.EncodeToString(jtiBytes)

	expiresAt := time.Now().Add(s.jwtExpiration)

	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "terraform-mirror",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, jti, expiresAt, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GenerateRandomPassword generates a secure random password
func GenerateRandomPassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}

	// Convert to base64 for readability (will be longer than length)
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
