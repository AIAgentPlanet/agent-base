package ath

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/model"
)

const (
	jtiPrefix         = "ath:jti:"
	sessionPrefix     = "ath:session:"
	attestationMaxAge = 5 * time.Minute
	jtiTTL            = 10 * time.Minute
	sessionTTL        = 10 * time.Minute
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

// AttestationClaims represents the expected claims in an agent attestation JWT
type AttestationClaims struct {
	AgentID string `json:"agent_id,omitempty"`
	jwt.RegisteredClaims
}

// VerifyAttestation verifies an ES256 attestation JWT against an agent's registered public key.
// If agent is nil, it parses the JWT unverified to extract agent_id, and the caller must
// look up the agent and call VerifyAttestation again with the agent.
func VerifyAttestation(tokenString string, agent *model.ATHAgent) (*AttestationClaims, error) {
	if agent == nil {
		claims := &AttestationClaims{}
		_, _, err := new(jwt.Parser).ParseUnverified(tokenString, claims)
		if err != nil {
			return nil, fmt.Errorf("invalid attestation format: %w", err)
		}
		return claims, nil
	}

	publicKey, err := parseECPublicKey(agent.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid agent public key: %w", err)
	}

	claims := &AttestationClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("attestation verification failed: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("attestation token invalid")
	}

	// Validate freshness: iat must be within last 5 minutes
	if claims.IssuedAt == nil {
		return nil, fmt.Errorf("attestation missing iat")
	}
	if time.Since(claims.IssuedAt.Time) > attestationMaxAge {
		return nil, fmt.Errorf("attestation too old")
	}
	if claims.IssuedAt.Time.After(time.Now().Add(1 * time.Minute)) {
		return nil, fmt.Errorf("attestation issued in the future")
	}

	// Validate jti presence
	if claims.ID == "" {
		return nil, fmt.Errorf("attestation missing jti")
	}

	// Validate sub matches agent_id
	if claims.Subject == "" {
		return nil, fmt.Errorf("attestation missing sub")
	}
	if claims.Subject != agent.AgentID {
		return nil, fmt.Errorf("attestation sub mismatch")
	}

	return claims, nil
}

// CheckJTIReplay checks if a jti has been used before, and records it if not.
func CheckJTIReplay(jti string) error {
	if database.GetRedisCli() == nil {
		return fmt.Errorf("redis is required for ath jti replay protection")
	}
	key := jtiPrefix + jti
	ok, err := database.GetRedisCli().SetNX(context.Background(), key, "1", jtiTTL).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("jti replay detected")
	}
	return nil
}

// SaveSession stores an ath_session_id in Redis with TTL.
func SaveSession(sessionID string, data string) error {
	if database.GetRedisCli() == nil {
		return fmt.Errorf("redis is required for ath sessions")
	}
	return database.GetRedisCli().Set(context.Background(), sessionPrefix+sessionID, data, sessionTTL).Err()
}

// GetSession retrieves and optionally deletes an ath_session_id from Redis.
func GetSession(sessionID string, consume bool) (string, error) {
	if database.GetRedisCli() == nil {
		return "", fmt.Errorf("redis is required for ath sessions")
	}
	key := sessionPrefix + sessionID
	val, err := database.GetRedisCli().Get(context.Background(), key).Result()
	if err != nil {
		return "", err
	}
	if consume {
		_ = database.GetRedisCli().Del(context.Background(), key).Err()
	}
	return val, nil
}

// GenerateClientID generates a random client id for ATH agents.
func GenerateClientID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 24)
	for i := range b {
		b[i] = chars[rnd.Intn(len(chars))]
	}
	return string(b)
}

// GenerateClientSecret generates a random client secret for ATH agents.
func GenerateClientSecret() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	b := make([]byte, 48)
	for i := range b {
		b[i] = chars[rnd.Intn(len(chars))]
	}
	return string(b)
}

// GenerateSessionID generates a random session id.
func GenerateSessionID() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = chars[rnd.Intn(len(chars))]
	}
	return string(b)
}

// GenerateState generates a random state parameter with at least 128 bits of entropy.
func GenerateState() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = chars[rnd.Intn(len(chars))]
	}
	return string(b)
}

func parseECPublicKey(pemKey string) (*ecdsa.PublicKey, error) {
	pemKey = strings.TrimSpace(pemKey)
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}
	return ecPub, nil
}
