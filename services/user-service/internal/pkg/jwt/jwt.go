package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	secretKey   = []byte("user-service-default-secret-key-change-me")
	issuer      = "user-service"
	expireHours = 24
)

// Claims custom JWT claims
type Claims struct {
	UserID      uint64 `json:"user_id"`
	ClientID    string `json:"client_id,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	ProviderID  string `json:"provider_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	HandshakeID string `json:"handshake_id,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Type        string `json:"type,omitempty"`
	jwt.RegisteredClaims
}

// OAuthClaims alias for Claims
type OAuthClaims = Claims

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

// GenerateToken generate JWT token for user login
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

// ParseToken parse and validate JWT token, return userID
func ParseToken(tokenString string) (uint64, error) {
	claims, err := parseToken(tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

// GenerateOAuthToken generate OAuth access/refresh token
func GenerateOAuthToken(userID uint64, clientID string, scope string, tokenType string, expireSeconds int) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		ClientID: clientID,
		Scope:    scope,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        generateJTI(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireSeconds) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

// GenerateATHToken generates an ATH access/refresh token bound to (agent_id, user_id, scope).
func GenerateATHToken(userID uint64, agentID string, clientID string, providerID string, sessionID string, handshakeID string, scope string, tokenType string, expireSeconds int) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:      userID,
		ClientID:    clientID,
		AgentID:     agentID,
		ProviderID:  providerID,
		SessionID:   sessionID,
		HandshakeID: handshakeID,
		Scope:       scope,
		Type:        "ath_" + tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        generateJTI(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireSeconds) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	claims.Audience = jwt.ClaimStrings{providerID}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

// ParseATHToken parses and validates an ATH token, ensuring type starts with "ath_".
func ParseATHToken(tokenString string) (*Claims, error) {
	claims, err := parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Type == "" || (claims.Type != "ath_access" && claims.Type != "ath_refresh") {
		return nil, fmt.Errorf("not an ATH token")
	}
	return claims, nil
}

// ParseOAuthToken parse and validate OAuth token, return full claims
func ParseOAuthToken(tokenString string) (*Claims, error) {
	return parseToken(tokenString)
}

func parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func generateJTI() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// GetAgentIDFromClaims extracts agent_id from the Audience field of ATH claims.
func GetAgentIDFromClaims(claims *Claims) string {
	return claims.AgentID
}
