package ath

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	requestNoncePrefix = "ath:request-nonce:"
	requestFreshness   = 5 * time.Minute
)

var ErrRequestReplay = errors.New("ath request replay detected")

type IntegrityInput struct {
	HandshakeID string
	TokenID     string
	Provider    string
	Method      string
	Path        string
	Body        string
	Timestamp   int64
	Nonce       string
	Signature   string
}

type IntegrityPayload struct {
	Type        string `json:"type"`
	HandshakeID string `json:"handshake_id"`
	TokenID     string `json:"token_jti"`
	Provider    string `json:"provider"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	BodySHA256  string `json:"body_sha256"`
	Timestamp   int64  `json:"timestamp"`
	Nonce       string `json:"nonce"`
}

type RequestReplayStore interface {
	Use(ctx context.Context, handshakeID, nonce string, ttl time.Duration) error
}

type RedisRequestReplayStore struct {
	client *redis.Client
}

func NewRedisRequestReplayStore(client *redis.Client) *RedisRequestReplayStore {
	return &RedisRequestReplayStore{client: client}
}

func (s *RedisRequestReplayStore) Use(ctx context.Context, handshakeID, nonce string, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis is required for ATH request replay protection")
	}
	nonceHash := sha256.Sum256([]byte(nonce))
	key := requestNoncePrefix + handshakeID + ":" + hex.EncodeToString(nonceHash[:])
	ok, err := s.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrRequestReplay
	}
	return nil
}

func (s *HandshakeService) SetRequestReplayStore(store RequestReplayStore) {
	s.replayStore = store
}

func (s *HandshakeService) VerifyRequestIntegrity(ctx context.Context, input IntegrityInput) error {
	handshake, err := s.Get(ctx, input.HandshakeID)
	if err != nil {
		return err
	}
	if handshake.State != HandshakeStateIdentityVerified || handshake.SessionKey == "" {
		return fmt.Errorf("ATH session key is not established")
	}
	now := s.now()
	if err := validateIntegrityTimestamp(now, input.Timestamp); err != nil {
		return err
	}
	nonce, err := base64.RawURLEncoding.DecodeString(input.Nonce)
	if err != nil || len(nonce) < 16 {
		return fmt.Errorf("request nonce must be base64url with at least 128 bits")
	}
	key, err := base64.RawURLEncoding.DecodeString(handshake.SessionKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("invalid ATH session key")
	}
	expected, err := SignIntegrityInput(key, input)
	if err != nil {
		return err
	}
	provided, err := base64.RawURLEncoding.DecodeString(input.Signature)
	if err != nil {
		return fmt.Errorf("invalid request signature encoding")
	}
	expectedBytes, _ := base64.RawURLEncoding.DecodeString(expected)
	if !hmac.Equal(expectedBytes, provided) {
		return fmt.Errorf("request integrity verification failed")
	}
	if s.replayStore == nil {
		return fmt.Errorf("ATH request replay store is not configured")
	}
	ttl := time.Unix(handshake.ExpiresAt, 0).Sub(now)
	if ttl <= 0 {
		return ErrHandshakeNotFound
	}
	return s.replayStore.Use(ctx, handshake.ID, input.Nonce, ttl)
}

func SignIntegrityInput(sessionKey []byte, input IntegrityInput) (string, error) {
	if len(sessionKey) != 32 {
		return "", fmt.Errorf("ATH session key must be 32 bytes")
	}
	payload := canonicalIntegrityPayload(input)
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, sessionKey)
	_, _ = mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func canonicalIntegrityPayload(input IntegrityInput) IntegrityPayload {
	bodyDigest := sha256.Sum256([]byte(input.Body))
	return IntegrityPayload{
		Type: "ath_request_integrity", HandshakeID: input.HandshakeID,
		TokenID: input.TokenID, Provider: input.Provider,
		Method: strings.ToUpper(input.Method), Path: input.Path,
		BodySHA256: hex.EncodeToString(bodyDigest[:]),
		Timestamp:  input.Timestamp, Nonce: input.Nonce,
	}
}

func validateIntegrityTimestamp(now time.Time, timestamp int64) error {
	if timestamp == 0 {
		return fmt.Errorf("request timestamp is required")
	}
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < -time.Minute || delta > requestFreshness {
		return fmt.Errorf("request timestamp is not fresh")
	}
	return nil
}
