package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	secretKey     = []byte("user-service-default-secret-key-change-me")
	issuer        = "user-service"
	expireHours   = 24
)

// Claims custom JWT claims
type Claims struct {
	UserID uint64 `json:"user_id"`
	jwt.RegisteredClaims
}

// SetConfig set jwt config
func SetConfig(secret string, iss string, hours int) {
	if secret != "" {
		secretKey = []byte(secret)
	}
	if iss != "" {
		issuer = iss
	}
	if hours > 0 {
		expireHours = hours
	}
}

// GenerateToken generate JWT token
func GenerateToken(userID uint64) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

// ParseToken parse and validate JWT token
func ParseToken(tokenString string) (uint64, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})
	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims.UserID, nil
	}
	return 0, fmt.Errorf("invalid token")
}
