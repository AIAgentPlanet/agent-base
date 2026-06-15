package ath

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type memoryHandshakeStore struct {
	mu         sync.Mutex
	handshakes map[string]*Handshake
}

func newMemoryHandshakeStore() *memoryHandshakeStore {
	return &memoryHandshakeStore{handshakes: make(map[string]*Handshake)}
}

func (s *memoryHandshakeStore) Create(_ context.Context, handshake *Handshake, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.handshakes[handshake.ID]; exists {
		return ErrHandshakeStateConflict
	}
	copy := *handshake
	s.handshakes[handshake.ID] = &copy
	return nil
}

func (s *memoryHandshakeStore) Get(_ context.Context, id string) (*Handshake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handshake, exists := s.handshakes[id]
	if !exists {
		return nil, ErrHandshakeNotFound
	}
	copy := *handshake
	return &copy, nil
}

func (s *memoryHandshakeStore) Transition(_ context.Context, handshake *Handshake, expectedState string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.handshakes[handshake.ID]
	if !exists {
		return ErrHandshakeNotFound
	}
	if current.State != expectedState {
		return ErrHandshakeStateConflict
	}
	copy := *handshake
	s.handshakes[handshake.ID] = &copy
	return nil
}

func TestHandshakeMutualIdentityVerification(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientPEM, err := marshalECPublicKey(&clientKey.PublicKey)
	require.NoError(t, err)
	clientEphemeral, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := NewServerSigner("did:web:gateway.example.com", "", serverKey)
	require.NoError(t, err)

	service := NewHandshakeService(newMemoryHandshakeStore(), signer, time.Minute)
	service.now = func() time.Time { return now }
	clientNonce := make([]byte, 32)
	_, err = rand.Read(clientNonce)
	require.NoError(t, err)

	handshake, err := service.Start(context.Background(), StartHandshakeInput{
		ClientID: "client-1", ClientDID: "did:web:agent.example.com",
		Versions: []string{"0.1"},
		Capabilities: []string{
			"ES256", "PKCE-S256", "ECDH-P256", "HKDF-SHA256", "HMAC-SHA256",
		},
		Nonce:        base64.RawURLEncoding.EncodeToString(clientNonce),
		EphemeralKey: base64.RawURLEncoding.EncodeToString(clientEphemeral.PublicKey().Bytes()),
		Timestamp:    now.Unix(),
	})
	require.NoError(t, err)
	require.Equal(t, HandshakeStateChallengeIssued, handshake.State)
	identityDocument, err := service.ServerIdentityDocument()
	require.NoError(t, err)
	require.NoError(t, identityDocument.Validate("did:web:gateway.example.com"))

	serverPayload, err := json.Marshal(ServerChallengePayload{
		Type: serverChallengeType, HandshakeID: handshake.ID,
		ClientDID: handshake.ClientDID, ServerDID: handshake.ServerDID,
		ClientNonce: handshake.ClientNonce, ServerNonce: handshake.ServerNonce,
		ClientEphemeralKey: handshake.ClientEphemeralKey,
		ServerEphemeralKey: handshake.ServerEphemeralKey,
		Version:            handshake.Version, Timestamp: handshake.CreatedAt,
	})
	require.NoError(t, err)
	serverSignature, err := base64.RawURLEncoding.DecodeString(handshake.ServerSignature)
	require.NoError(t, err)
	serverDigest := sha256.Sum256(serverPayload)
	require.True(t, ecdsa.VerifyASN1(&serverKey.PublicKey, serverDigest[:], serverSignature))

	proofPayload, err := json.Marshal(ClientIdentityProofPayload{
		Type: clientIdentityProofType, HandshakeID: handshake.ID,
		ClientDID: handshake.ClientDID, ServerDID: handshake.ServerDID,
		ServerNonce: handshake.ServerNonce, Version: handshake.Version,
		ClientEphemeralKey: handshake.ClientEphemeralKey,
		ServerEphemeralKey: handshake.ServerEphemeralKey,
		Timestamp:          now.Unix(),
	})
	require.NoError(t, err)
	proofDigest := sha256.Sum256(proofPayload)
	proof, err := ecdsa.SignASN1(rand.Reader, clientKey, proofDigest[:])
	require.NoError(t, err)

	completed, err := service.Complete(context.Background(), CompleteHandshakeInput{
		HandshakeID: handshake.ID, ClientID: "client-1",
		ClientDID: "did:web:agent.example.com", PublicKeyPEM: clientPEM,
		Signature: base64.RawURLEncoding.EncodeToString(proof), Timestamp: now.Unix(),
	})
	require.NoError(t, err)
	require.Equal(t, HandshakeStateIdentityVerified, completed.State)
	require.Equal(t, now.Unix(), completed.VerifiedAt)
	serverEphemeralBytes, err := base64.RawURLEncoding.DecodeString(completed.ServerEphemeralKey)
	require.NoError(t, err)
	serverEphemeral, err := ecdh.P256().NewPublicKey(serverEphemeralBytes)
	require.NoError(t, err)
	clientSharedSecret, err := clientEphemeral.ECDH(serverEphemeral)
	require.NoError(t, err)
	clientSessionKey, err := deriveSessionKey(clientSharedSecret, completed)
	require.NoError(t, err)
	require.Equal(t, base64.RawURLEncoding.EncodeToString(clientSessionKey), completed.SessionKey)

	_, err = service.RequireVerified(context.Background(), handshake.ID, handshake.ClientDID, handshake.ClientID)
	require.NoError(t, err)
	_, err = service.Complete(context.Background(), CompleteHandshakeInput{
		HandshakeID: handshake.ID, ClientID: "client-1",
		ClientDID: handshake.ClientDID, PublicKeyPEM: clientPEM,
		Signature: base64.RawURLEncoding.EncodeToString(proof), Timestamp: now.Unix(),
	})
	require.ErrorIs(t, err, ErrHandshakeStateConflict)
}

func TestHandshakeRejectsTamperedClientProof(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientPEM, err := marshalECPublicKey(&clientKey.PublicKey)
	require.NoError(t, err)
	clientEphemeral, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := NewServerSigner("did:web:gateway.example.com", "", serverKey)
	require.NoError(t, err)
	service := NewHandshakeService(newMemoryHandshakeStore(), signer, time.Minute)
	service.now = func() time.Time { return now }

	handshake, err := service.Start(context.Background(), StartHandshakeInput{
		ClientID: "client-1", ClientDID: "did:web:agent.example.com",
		Versions:     []string{"0.1"},
		Capabilities: []string{"ES256", "ECDH-P256", "HKDF-SHA256", "HMAC-SHA256"},
		Nonce:        base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		EphemeralKey: base64.RawURLEncoding.EncodeToString(clientEphemeral.PublicKey().Bytes()),
		Timestamp:    now.Unix(),
	})
	require.NoError(t, err)
	payload, err := json.Marshal(ClientIdentityProofPayload{
		Type: clientIdentityProofType, HandshakeID: handshake.ID,
		ClientDID: handshake.ClientDID, ServerDID: handshake.ServerDID,
		ServerNonce: "tampered", Version: handshake.Version, Timestamp: now.Unix(),
		ClientEphemeralKey: handshake.ClientEphemeralKey,
		ServerEphemeralKey: handshake.ServerEphemeralKey,
	})
	require.NoError(t, err)
	digest := sha256.Sum256(payload)
	signature, err := ecdsa.SignASN1(rand.Reader, clientKey, digest[:])
	require.NoError(t, err)

	_, err = service.Complete(context.Background(), CompleteHandshakeInput{
		HandshakeID: handshake.ID, ClientID: handshake.ClientID,
		ClientDID: handshake.ClientDID, PublicKeyPEM: clientPEM,
		Signature: base64.RawURLEncoding.EncodeToString(signature), Timestamp: now.Unix(),
	})
	require.ErrorContains(t, err, "signature verification failed")
}

func TestHandshakeRejectsStaleTimestampAndWeakNonce(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	signer, err := NewServerSigner("did:web:gateway.example.com", "", serverKey)
	require.NoError(t, err)
	service := NewHandshakeService(newMemoryHandshakeStore(), signer, time.Minute)
	service.now = func() time.Time { return now }

	_, err = service.Start(context.Background(), StartHandshakeInput{
		ClientID: "client-1", ClientDID: "did:web:agent.example.com",
		Versions: []string{"0.1"}, Capabilities: []string{"ES256"},
		Nonce:     base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		Timestamp: now.Add(-10 * time.Minute).Unix(),
	})
	require.ErrorContains(t, err, "not fresh")

	_, err = service.Start(context.Background(), StartHandshakeInput{
		ClientID: "client-1", ClientDID: "did:web:agent.example.com",
		Versions: []string{"0.1"}, Capabilities: []string{"ES256"},
		Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 16)), Timestamp: now.Unix(),
	})
	require.ErrorContains(t, err, "256 bits")
}

func TestRedisHandshakeStoreAtomicTransition(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRedisHandshakeStore(client)
	ctx := context.Background()
	handshake := &Handshake{
		ID: "handshake-1", State: HandshakeStateChallengeIssued,
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}

	require.NoError(t, store.Create(ctx, handshake, time.Minute))
	handshake.State = HandshakeStateIdentityVerified
	require.NoError(t, store.Transition(ctx, handshake, HandshakeStateChallengeIssued, time.Hour))
	require.ErrorIs(t, store.Transition(ctx, handshake, HandshakeStateChallengeIssued, time.Hour), ErrHandshakeStateConflict)
	require.Equal(t, time.Hour, server.TTL(handshakePrefix+handshake.ID))

	stored, err := store.Get(ctx, handshake.ID)
	require.NoError(t, err)
	require.Equal(t, HandshakeStateIdentityVerified, stored.State)
}

func TestServerKeyRingPublishesHistoricalKeysAndUsesActiveKey(t *testing.T) {
	did := "did:web:gateway.example.com"
	var configs []SigningKeyConfig
	for _, keyID := range []string{did + "#key-2025", did + "#key-2026"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		path := t.TempDir() + "/key.pem"
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600))
		configs = append(configs, SigningKeyConfig{ID: keyID, KeyFile: path})
	}

	signer, err := LoadServerKeyRing(did, did+"#key-2026", configs, false)
	require.NoError(t, err)
	require.Equal(t, did+"#key-2026", signer.KeyID())
	require.Len(t, signer.VerificationMethods(), 2)
	require.Equal(t, did+"#key-2025", signer.VerificationMethods()[0].ID)
	require.Equal(t, did+"#key-2026", signer.VerificationMethods()[1].ID)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRemoteSignerVerifiesKMSResponse(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	publicPEM, err := marshalECPublicKey(&key.PublicKey)
	require.NoError(t, err)
	signer := &ServerSigner{
		did: "did:web:gateway.example.com", keyID: "key-remote",
		publicPEM: publicPEM, remoteEndpoint: "https://kms.example.com/sign",
		remoteAuthToken: "secret",
	}
	signer.remoteClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer secret", req.Header.Get("Authorization"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		digest, decodeErr := base64.RawURLEncoding.DecodeString(body["digest"])
		require.NoError(t, decodeErr)
		signature, signErr := ecdsa.SignASN1(rand.Reader, key, digest)
		require.NoError(t, signErr)
		response, marshalErr := json.Marshal(map[string]string{
			"signature": base64.RawURLEncoding.EncodeToString(signature),
		})
		require.NoError(t, marshalErr)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(response)),
			Header:     make(http.Header),
		}, nil
	})}

	digest := sha256.Sum256([]byte("audit-record"))
	signature, err := signer.SignDigest(digest[:])
	require.NoError(t, err)
	require.NotEmpty(t, signature)
}
