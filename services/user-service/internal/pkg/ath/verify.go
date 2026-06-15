package ath

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
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

// AttestationClaims represents the expected claims in an agent attestation JWT
type AttestationClaims struct {
	AgentID string `json:"agent_id,omitempty"`
	jwt.RegisteredClaims
}

type AttestationVerifier struct {
	Resolver IdentityResolver
	Now      func() time.Time
}

type VerifiedAttestation struct {
	Claims       *AttestationClaims
	Document     *IdentityDocument
	PublicKeyPEM string
	KeyID        string
}

func NewAttestationVerifier(resolver IdentityResolver) *AttestationVerifier {
	return &AttestationVerifier{Resolver: resolver, Now: time.Now}
}

func (v *AttestationVerifier) VerifyRegistration(ctx context.Context, tokenString, agentID, audience string) (*VerifiedAttestation, error) {
	if v.Resolver == nil {
		return nil, fmt.Errorf("identity resolver is required")
	}
	kid, err := tokenKeyID(tokenString)
	if err != nil {
		return nil, err
	}
	document, err := v.Resolver.Resolve(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("resolve agent identity: %w", err)
	}
	if err := document.Validate(agentID); err != nil {
		return nil, err
	}
	publicKey, resolvedKid, err := document.VerificationKey(kid)
	if err != nil {
		return nil, err
	}
	claims, err := v.verifyWithKey(tokenString, publicKey, agentID, audience)
	if err != nil {
		return nil, err
	}
	publicKeyPEM, err := marshalECPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	return &VerifiedAttestation{
		Claims: claims, Document: document, PublicKeyPEM: publicKeyPEM, KeyID: resolvedKid,
	}, nil
}

func (v *AttestationVerifier) VerifyRegistered(tokenString string, agent *model.ATHAgent, audience string) (*AttestationClaims, error) {
	if agent == nil {
		return nil, fmt.Errorf("registered agent is required")
	}
	publicKey, err := parseECPublicKey(agent.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid agent public key: %w", err)
	}
	return v.verifyWithKey(tokenString, publicKey, agent.AgentID, audience)
}

func (v *AttestationVerifier) verifyWithKey(tokenString string, publicKey *ecdsa.PublicKey, agentID, audience string) (*AttestationClaims, error) {
	claims := &AttestationClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithAudience(audience),
		jwt.WithIssuer(agentID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(time.Minute),
	)
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("attestation verification failed: %w", err)
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	if claims.IssuedAt == nil || now.Sub(claims.IssuedAt.Time) > attestationMaxAge {
		return nil, fmt.Errorf("attestation is not fresh")
	}
	if claims.IssuedAt.Time.After(now.Add(time.Minute)) {
		return nil, fmt.Errorf("attestation issued in the future")
	}
	if claims.ID == "" {
		return nil, fmt.Errorf("attestation missing jti")
	}
	if claims.Subject != agentID {
		return nil, fmt.Errorf("attestation sub mismatch")
	}
	if claims.Issuer != agentID {
		return nil, fmt.Errorf("attestation iss mismatch")
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

func DeleteSession(sessionID string) error {
	if database.GetRedisCli() == nil {
		return fmt.Errorf("redis is required for ath sessions")
	}
	return database.GetRedisCli().Del(context.Background(), sessionPrefix+sessionID).Err()
}

// GenerateClientID generates a random client id for ATH agents.
func GenerateClientID() string {
	return secureRandomString(24)
}

// GenerateClientSecret generates a random client secret for ATH agents.
func GenerateClientSecret() string {
	return secureRandomString(48)
}

// GenerateSessionID generates a random session id.
func GenerateSessionID() string {
	return secureRandomString(32)
}

// GenerateState generates a random state parameter with at least 128 bits of entropy.
func GenerateState() string {
	return secureRandomString(32)
}

func secureRandomString(byteCount int) string {
	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func tokenKeyID(tokenString string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &AttestationClaims{})
	if err != nil {
		return "", fmt.Errorf("invalid attestation format: %w", err)
	}
	alg, _ := token.Header["alg"].(string)
	if alg != jwt.SigningMethodES256.Alg() {
		return "", fmt.Errorf("attestation algorithm must be ES256")
	}
	kid, _ := token.Header["kid"].(string)
	return kid, nil
}

func marshalECPublicKey(key *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
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
