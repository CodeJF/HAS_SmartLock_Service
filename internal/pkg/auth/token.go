package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

type AccessClaims struct {
	UID string `json:"uid"`
	jwt.RegisteredClaims
}

func NewTokenManager(secret string, accessTokenTTLSeconds, refreshTokenTTLSeconds int) *TokenManager {
	return &TokenManager{
		secret:          []byte(secret),
		accessTokenTTL:  time.Duration(accessTokenTTLSeconds) * time.Second,
		refreshTokenTTL: time.Duration(refreshTokenTTLSeconds) * time.Second,
	}
}

func (m *TokenManager) AccessTokenExpiresAt(now time.Time) time.Time {
	return now.Add(m.accessTokenTTL)
}

func (m *TokenManager) RefreshTokenExpiresAt(now time.Time) time.Time {
	return now.Add(m.refreshTokenTTL)
}

func (m *TokenManager) GenerateAccessToken(uid string, now time.Time) (string, time.Time, error) {
	expiresAt := m.AccessTokenExpiresAt(now)
	claims := AccessClaims{
		UID: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uid,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (m *TokenManager) ParseAccessToken(raw string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &AccessClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid access token")
	}

	return claims, nil
}

func GenerateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
