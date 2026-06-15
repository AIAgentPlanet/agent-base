package ath

import (
	"context"
	"encoding/base64"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type memoryReplayStore struct {
	mu     sync.Mutex
	nonces map[string]bool
}

func (s *memoryReplayStore) Use(_ context.Context, handshakeID, nonce string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := handshakeID + ":" + nonce
	if s.nonces[key] {
		return ErrRequestReplay
	}
	s.nonces[key] = true
	return nil
}

func TestRequestIntegrityVerificationAndReplayProtection(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	store := newMemoryHandshakeStore()
	handshake := &Handshake{
		ID: "handshake-1", ClientID: "client-1",
		State:      HandshakeStateIdentityVerified,
		SessionKey: base64.RawURLEncoding.EncodeToString(key),
		ExpiresAt:  now.Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Create(context.Background(), handshake, time.Hour))
	service := NewHandshakeService(store, nil, time.Minute)
	service.now = func() time.Time { return now }
	service.SetRequestReplayStore(&memoryReplayStore{nonces: make(map[string]bool)})

	input := IntegrityInput{
		HandshakeID: handshake.ID, TokenID: "token-jti-1",
		Provider: "user-service", Method: "POST", Path: "/api/v1/users",
		Body: `{"name":"alice"}`, Timestamp: now.Unix(),
		Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
	}
	signature, err := SignIntegrityInput(key, input)
	require.NoError(t, err)
	input.Signature = signature

	require.NoError(t, service.VerifyRequestIntegrity(context.Background(), input))
	require.ErrorIs(t, service.VerifyRequestIntegrity(context.Background(), input), ErrRequestReplay)
}

func TestRequestIntegrityRejectsTamperingAndStaleTimestamp(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	key := make([]byte, 32)
	store := newMemoryHandshakeStore()
	handshake := &Handshake{
		ID: "handshake-1", State: HandshakeStateIdentityVerified,
		SessionKey: base64.RawURLEncoding.EncodeToString(key),
		ExpiresAt:  now.Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Create(context.Background(), handshake, time.Hour))
	service := NewHandshakeService(store, nil, time.Minute)
	service.now = func() time.Time { return now }
	service.SetRequestReplayStore(&memoryReplayStore{nonces: make(map[string]bool)})

	input := IntegrityInput{
		HandshakeID: handshake.ID, TokenID: "token-jti-1",
		Provider: "user-service", Method: "POST", Path: "/resource",
		Body: "original", Timestamp: now.Unix(),
		Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
	}
	signature, err := SignIntegrityInput(key, input)
	require.NoError(t, err)
	input.Signature = signature
	input.Body = "tampered"
	require.ErrorContains(t, service.VerifyRequestIntegrity(context.Background(), input), "verification failed")

	input.Body = "original"
	input.Timestamp = now.Add(-10 * time.Minute).Unix()
	require.ErrorContains(t, service.VerifyRequestIntegrity(context.Background(), input), "not fresh")
}

func TestIntegrityPayloadIsDeterministic(t *testing.T) {
	key := make([]byte, 32)
	input := IntegrityInput{
		HandshakeID: "h1", TokenID: "j1", Provider: "user-service",
		Method: "get", Path: "/resource", Body: "", Timestamp: 123,
		Nonce: "nonce",
	}
	first, err := SignIntegrityInput(key, input)
	require.NoError(t, err)
	second, err := SignIntegrityInput(key, input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, "GET", canonicalIntegrityPayload(input).Method)
}

func TestRedisRequestReplayStore(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRedisRequestReplayStore(client)

	require.NoError(t, store.Use(context.Background(), "handshake-1", "nonce-1", time.Minute))
	require.ErrorIs(
		t,
		store.Use(context.Background(), "handshake-1", "nonce-1", time.Minute),
		ErrRequestReplay,
	)
	require.NoError(t, store.Use(context.Background(), "handshake-1", "nonce-2", time.Minute))
}
