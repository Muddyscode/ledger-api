package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func IssueToken(secret, email string, operatorID uuid.UUID, ttl time.Duration) (string, time.Time, error) {
	exp := time.Now().Add(ttl)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   operatorID.String(),
			Issuer:    "ledger-api",
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	s, err := t.SignedString([]byte(secret))
	return s, exp, err
}

func ParseToken(secret, token string) (uuid.UUID, string, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return uuid.Nil, "", fmt.Errorf("invalid token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", err
	}
	return id, claims.Email, nil
}
