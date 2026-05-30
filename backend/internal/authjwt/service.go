package authjwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type TokenClaims struct {
	SubjectID   string `json:"sub"`
	SubjectType string `json:"role"`
	jwt.RegisteredClaims
}

type TokenService interface {
	GenerateAccessToken(subjectID string, subjectType string) (string, error)
	VerifyAccessToken(tokenString string) (*TokenClaims, error)
	GenerateRefreshToken() (plain string, hashed string, err error)
}

type hs256Service struct {
	secret       []byte
	accessExpiry time.Duration
}

func NewHS256Service(secret string, accessExpiry time.Duration) TokenService {
	return &hs256Service{
		secret:       []byte(secret),
		accessExpiry: accessExpiry,
	}
}

func (s *hs256Service) GenerateAccessToken(subjectID string, subjectType string) (string, error) {
	now := time.Now().UTC()
	claims := TokenClaims{
		SubjectID:   subjectID,
		SubjectType: subjectType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bohikor2",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *hs256Service) VerifyAccessToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

func (s *hs256Service) GenerateRefreshToken() (plain string, hashed string, err error) {
	plain, err = generateOpaqueToken()
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	hashed = HashTokenStr(plain)
	return plain, hashed, nil
}
