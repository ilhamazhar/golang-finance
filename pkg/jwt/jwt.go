package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`       // authorization role, e.g. "user" or "admin"
	TokenType string    `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type Manager struct {
	secret    []byte
	expiry    time.Duration
	tokenType string
}

func NewManager(secret string, expiry time.Duration, tokenType string) *Manager {
	return &Manager{
		secret:    []byte(secret),
		expiry:    expiry,
		tokenType: tokenType,
	}
}

func (m *Manager) Expiry() time.Duration {
	return m.expiry
}

func (m *Manager) Generate(userID uuid.UUID, email, role string) (string, error) {
	claims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: m.tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) Verify(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
