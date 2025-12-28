package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid JWT token")
	ErrExpiredToken = errors.New("token has expired")
	ErrInvalidClaim = errors.New("invalid token claims")
)

// Manager handles JWT operations
type Manager struct{}

// NewManager creates a new JWT Manager
func NewManager() *Manager {
	return &Manager{}
}

// Sign creates a JWT token with the given claims
func (m *Manager) Sign(secret string, claims map[string]interface{}) (string, error) {
	jwtClaims := jwt.MapClaims{}
	for k, v := range claims {
		jwtClaims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	return token.SignedString([]byte(secret))
}

// SignWithExpiry creates a JWT token with expiration time
func (m *Manager) SignWithExpiry(secret string, claims map[string]interface{}, expiry time.Duration) (string, error) {
	jwtClaims := jwt.MapClaims{}
	for k, v := range claims {
		jwtClaims[k] = v
	}
	jwtClaims["exp"] = time.Now().Add(expiry).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	return token.SignedString([]byte(secret))
}

// Verify validates a JWT token and returns the claims
func (m *Manager) Verify(secret string, tokenString string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaim
	}

	return claims, nil
}

// GenerateSecret generates a 32-character random hex string for use as a JWT secret
func (m *Manager) GenerateSecret() string {
	bytes := make([]byte, 16) // 16 bytes = 32 hex characters
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less secure but functional approach
		// This should never happen in practice
		return "fallback-secret-key-do-not-use!"
	}
	return hex.EncodeToString(bytes)
}
