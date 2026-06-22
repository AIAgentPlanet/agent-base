package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var (
	ErrNotConfigured = errors.New("ATH JWT verifier is not configured")
	ErrInvalidToken  = errors.New("invalid ATH token")
)

type ATHJWTVerifier struct {
	secret []byte
	issuer string
	now    func() time.Time
}

type ATHClaims struct {
	UserID      uint64   `json:"user_id"`
	ClientID    string   `json:"client_id"`
	AgentID     string   `json:"agent_id"`
	ProviderID  string   `json:"provider_id"`
	SessionID   string   `json:"session_id"`
	HandshakeID string   `json:"handshake_id"`
	Scope       string   `json:"scope"`
	Type        string   `json:"type"`
	ID          string   `json:"jti"`
	Subject     string   `json:"sub"`
	Issuer      string   `json:"iss"`
	Audience    audience `json:"aud"`
	ExpiresAt   int64    `json:"exp"`
	NotBefore   int64    `json:"nbf"`
	IssuedAt    int64    `json:"iat"`
}

type VerifyInput struct {
	Token         string
	SessionID     string
	HandshakeID   string
	AgentIdentity string
	RequiredScope string
}

type Config struct {
	Secret string
	Issuer string
	Now    func() time.Time
}

func NewATHJWTVerifier(config Config) *ATHJWTVerifier {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &ATHJWTVerifier{secret: []byte(config.Secret), issuer: config.Issuer, now: now}
}

func (v *ATHJWTVerifier) Configured() bool {
	return v != nil && len(v.secret) > 0
}

func (v *ATHJWTVerifier) Verify(input VerifyInput) (*ATHClaims, error) {
	if !v.Configured() {
		return nil, ErrNotConfigured
	}
	claims, err := v.parse(input.Token)
	if err != nil {
		return nil, err
	}
	if claims.Type != "ath_access" {
		return nil, fmt.Errorf("%w: token type must be ath_access", ErrInvalidToken)
	}
	if v.issuer != "" && claims.Issuer != v.issuer {
		return nil, fmt.Errorf("%w: issuer mismatch", ErrInvalidToken)
	}
	now := v.now().Unix()
	if claims.ExpiresAt <= now {
		return nil, fmt.Errorf("%w: token expired", ErrInvalidToken)
	}
	if claims.NotBefore != 0 && claims.NotBefore > now {
		return nil, fmt.Errorf("%w: token not valid yet", ErrInvalidToken)
	}
	if input.SessionID != "" && claims.SessionID != input.SessionID {
		return nil, fmt.Errorf("%w: session mismatch", ErrInvalidToken)
	}
	if input.HandshakeID != "" && claims.HandshakeID != input.HandshakeID {
		return nil, fmt.Errorf("%w: handshake mismatch", ErrInvalidToken)
	}
	if input.AgentIdentity != "" && claims.AgentID != input.AgentIdentity {
		return nil, fmt.Errorf("%w: agent identity mismatch", ErrInvalidToken)
	}
	if input.RequiredScope != "" && !hasScope(claims.Scope, input.RequiredScope) {
		return nil, fmt.Errorf("%w: missing required scope", ErrInvalidToken)
	}
	return claims, nil
}

func (v *ATHJWTVerifier) parse(token string) (*ATHClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrInvalidToken
	}
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("%w: unexpected alg", ErrInvalidToken)
	}
	signed := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write([]byte(signed))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, fmt.Errorf("%w: signature mismatch", ErrInvalidToken)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims ATHClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func hasScope(scopeString, required string) bool {
	return slices.Contains(strings.Fields(scopeString), required)
}

type audience []string

func (a *audience) UnmarshalJSON(data []byte) error {
	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		*a = many
		return nil
	}
	var one string
	if err := json.Unmarshal(data, &one); err != nil {
		return err
	}
	*a = []string{one}
	return nil
}
