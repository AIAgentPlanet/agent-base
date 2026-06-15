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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/hkdf"
)

const (
	HandshakeStateChallengeIssued  = "challenge_issued"
	HandshakeStateIdentityVerified = "identity_verified"

	handshakePrefix         = "ath:handshake:"
	defaultHandshakeTTL     = 10 * time.Minute
	defaultSessionTTL       = time.Hour
	handshakeFreshness      = 5 * time.Minute
	handshakeVersion        = "0.1"
	serverChallengeType     = "server_challenge"
	clientIdentityProofType = "client_identity_proof"
)

var (
	ErrHandshakeNotFound      = errors.New("ath handshake not found")
	ErrHandshakeStateConflict = errors.New("ath handshake state conflict")
)

type Handshake struct {
	ID                 string   `json:"id"`
	ClientID           string   `json:"client_id"`
	ClientDID          string   `json:"client_did"`
	ServerDID          string   `json:"server_did"`
	Version            string   `json:"version"`
	Capabilities       []string `json:"capabilities"`
	ClientNonce        string   `json:"client_nonce"`
	ServerNonce        string   `json:"server_nonce"`
	ClientEphemeralKey string   `json:"client_ephemeral_key"`
	ServerEphemeralKey string   `json:"server_ephemeral_key"`
	SessionKey         string   `json:"session_key"`
	ServerKeyID        string   `json:"server_key_id"`
	ServerPublicKeyPEM string   `json:"server_public_key_pem"`
	ServerSignature    string   `json:"server_signature"`
	State              string   `json:"state"`
	CreatedAt          int64    `json:"created_at"`
	ExpiresAt          int64    `json:"expires_at"`
	VerifiedAt         int64    `json:"verified_at,omitempty"`
}

type StartHandshakeInput struct {
	ClientID     string
	ClientDID    string
	Versions     []string
	Capabilities []string
	Nonce        string
	EphemeralKey string
	Timestamp    int64
}

type CompleteHandshakeInput struct {
	HandshakeID  string
	ClientID     string
	ClientDID    string
	PublicKeyPEM string
	Signature    string
	Timestamp    int64
}

type ServerChallengePayload struct {
	Type               string `json:"type"`
	HandshakeID        string `json:"handshake_id"`
	ClientDID          string `json:"client_did"`
	ServerDID          string `json:"server_did"`
	ClientNonce        string `json:"client_nonce"`
	ServerNonce        string `json:"server_nonce"`
	ClientEphemeralKey string `json:"client_ephemeral_key"`
	ServerEphemeralKey string `json:"server_ephemeral_key"`
	Version            string `json:"version"`
	Timestamp          int64  `json:"timestamp"`
}

type ClientIdentityProofPayload struct {
	Type               string `json:"type"`
	HandshakeID        string `json:"handshake_id"`
	ClientDID          string `json:"client_did"`
	ServerDID          string `json:"server_did"`
	ServerNonce        string `json:"server_nonce"`
	ClientEphemeralKey string `json:"client_ephemeral_key"`
	ServerEphemeralKey string `json:"server_ephemeral_key"`
	Version            string `json:"version"`
	Timestamp          int64  `json:"timestamp"`
}

type HandshakeStore interface {
	Create(ctx context.Context, handshake *Handshake, ttl time.Duration) error
	Get(ctx context.Context, id string) (*Handshake, error)
	Transition(ctx context.Context, handshake *Handshake, expectedState string, ttl time.Duration) error
}

type RedisHandshakeStore struct {
	client *redis.Client
}

func NewRedisHandshakeStore(client *redis.Client) *RedisHandshakeStore {
	return &RedisHandshakeStore{client: client}
}

func (s *RedisHandshakeStore) Create(ctx context.Context, handshake *Handshake, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis is required for ath handshakes")
	}
	data, err := json.Marshal(handshake)
	if err != nil {
		return err
	}
	ok, err := s.client.SetNX(ctx, handshakePrefix+handshake.ID, data, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrHandshakeStateConflict
	}
	return nil
}

func (s *RedisHandshakeStore) Get(ctx context.Context, id string) (*Handshake, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("redis is required for ath handshakes")
	}
	data, err := s.client.Get(ctx, handshakePrefix+id).Bytes()
	if err == redis.Nil {
		return nil, ErrHandshakeNotFound
	}
	if err != nil {
		return nil, err
	}
	var handshake Handshake
	if err := json.Unmarshal(data, &handshake); err != nil {
		return nil, err
	}
	return &handshake, nil
}

func (s *RedisHandshakeStore) Transition(ctx context.Context, handshake *Handshake, expectedState string, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis is required for ath handshakes")
	}
	data, err := json.Marshal(handshake)
	if err != nil {
		return err
	}
	const transitionScript = `
local raw = redis.call("GET", KEYS[1])
if not raw then return 0 end
local current = cjson.decode(raw)
if current.state ~= ARGV[1] then return -1 end
redis.call("SET", KEYS[1], ARGV[2])
redis.call("PEXPIRE", KEYS[1], ARGV[3])
return 1
`
	result, err := s.client.Eval(
		ctx, transitionScript, []string{handshakePrefix + handshake.ID},
		expectedState, string(data), ttl.Milliseconds(),
	).Int()
	if err != nil {
		return err
	}
	switch result {
	case 0:
		return ErrHandshakeNotFound
	case -1:
		return ErrHandshakeStateConflict
	default:
		return nil
	}
}

type ServerSigner struct {
	did                 string
	keyID               string
	privateKey          *ecdsa.PrivateKey
	publicPEM           string
	verificationMethods []VerificationMethod
	remoteEndpoint      string
	remoteAuthToken     string
	remoteClient        *http.Client
}

func NewServerSigner(did, keyID string, privateKey *ecdsa.PrivateKey) (*ServerSigner, error) {
	if strings.TrimSpace(did) == "" {
		return nil, fmt.Errorf("server DID is required")
	}
	if !strings.HasPrefix(did, "did:") {
		return nil, fmt.Errorf("server identity must be a DID")
	}
	if privateKey == nil || privateKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("server signing key must be P-256")
	}
	if keyID == "" {
		keyID = did + "#key-1"
	}
	publicPEM, err := marshalECPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	return &ServerSigner{
		did: did, keyID: keyID, privateKey: privateKey, publicPEM: publicPEM,
		verificationMethods: []VerificationMethod{{
			ID: keyID, Type: "EcdsaSecp256r1VerificationKey2019",
			Controller: did, PublicKeyPEM: publicPEM,
		}},
	}, nil
}

type SigningKeyConfig struct {
	ID              string
	KeyFile         string
	PublicKeyFile   string
	SigningEndpoint string
	AuthToken       string
}

func LoadServerKeyRing(did, activeKeyID string, keys []SigningKeyConfig, allowEphemeral bool) (*ServerSigner, error) {
	if len(keys) == 0 {
		return LoadServerSigner(did, activeKeyID, "", allowEphemeral)
	}
	var active *ServerSigner
	methods := make([]VerificationMethod, 0, len(keys))
	seen := make(map[string]bool)
	for _, configured := range keys {
		if configured.ID == "" {
			return nil, fmt.Errorf("ATH signing key id is required")
		}
		if seen[configured.ID] {
			return nil, fmt.Errorf("duplicate ATH signing key id %q", configured.ID)
		}
		seen[configured.ID] = true
		signer, err := loadConfiguredServerSigner(did, configured, allowEphemeral)
		if err != nil {
			return nil, err
		}
		methods = append(methods, signer.verificationMethods[0])
		if configured.ID == activeKeyID {
			active = signer
		}
	}
	if activeKeyID == "" && len(keys) == 1 {
		activeKeyID = keys[0].ID
		var err error
		active, err = loadConfiguredServerSigner(did, keys[0], allowEphemeral)
		if err != nil {
			return nil, err
		}
	}
	if active == nil {
		return nil, fmt.Errorf("active ATH signing key %q was not found", activeKeyID)
	}
	active.verificationMethods = methods
	return active, nil
}

func loadConfiguredServerSigner(did string, configured SigningKeyConfig, allowHTTP bool) (*ServerSigner, error) {
	if configured.SigningEndpoint != "" {
		return LoadRemoteServerSigner(
			did, configured.ID, configured.PublicKeyFile,
			configured.SigningEndpoint, configured.AuthToken, allowHTTP,
		)
	}
	if configured.KeyFile == "" {
		return nil, fmt.Errorf("ATH signing key %q requires keyFile or signingEndpoint", configured.ID)
	}
	return LoadServerSigner(did, configured.ID, configured.KeyFile, false)
}

func LoadRemoteServerSigner(did, keyID, publicKeyFile, endpoint, authToken string, allowHTTP bool) (*ServerSigner, error) {
	if !strings.HasPrefix(did, "did:") {
		return nil, fmt.Errorf("server identity must be a DID")
	}
	if keyID == "" {
		return nil, fmt.Errorf("remote ATH signing key id is required")
	}
	if publicKeyFile == "" {
		return nil, fmt.Errorf("remote ATH signing key requires publicKeyFile")
	}
	data, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read ATH signing public key: %w", err)
	}
	publicKey, err := parseECPublicKey(string(data))
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid ATH remote signing endpoint")
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		return nil, fmt.Errorf("ATH remote signing endpoint must use HTTPS")
	}
	publicPEM, err := marshalECPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	return &ServerSigner{
		did: did, keyID: keyID, publicPEM: publicPEM,
		remoteEndpoint: endpoint, remoteAuthToken: authToken,
		remoteClient: &http.Client{Timeout: 10 * time.Second},
		verificationMethods: []VerificationMethod{{
			ID: keyID, Type: "EcdsaSecp256r1VerificationKey2019",
			Controller: did, PublicKeyPEM: publicPEM,
		}},
	}, nil
}

func LoadServerSigner(did, keyID, keyFile string, allowEphemeral bool) (*ServerSigner, error) {
	var privateKey *ecdsa.PrivateKey
	if keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("read ATH server signing key: %w", err)
		}
		privateKey, err = parseECPrivateKey(data)
		if err != nil {
			return nil, err
		}
	} else {
		if !allowEphemeral {
			return nil, fmt.Errorf("ath.signingKeyFile is required in production")
		}
		var err error
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ATH server signing key: %w", err)
		}
	}
	return NewServerSigner(did, keyID, privateKey)
}

func parseECPrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to parse ATH server signing key PEM")
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse ATH server signing key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("ATH server signing key is not ECDSA")
	}
	return key, nil
}

func (s *ServerSigner) Sign(payload interface{}) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return s.SignDigest(digest[:])
}

func (s *ServerSigner) SignDigest(digest []byte) (string, error) {
	if len(digest) != sha256.Size {
		return "", fmt.Errorf("digest must be SHA-256")
	}
	if s.privateKey == nil {
		return s.remoteSignDigest(digest)
	}
	signature, err := ecdsa.SignASN1(rand.Reader, s.privateKey, digest)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *ServerSigner) remoteSignDigest(digest []byte) (string, error) {
	if s.remoteClient == nil || s.remoteEndpoint == "" {
		return "", fmt.Errorf("ATH signer has no private key or remote signing endpoint")
	}
	body, err := json.Marshal(map[string]string{
		"key_id": s.keyID, "algorithm": "ES256",
		"digest": base64.RawURLEncoding.EncodeToString(digest),
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, s.remoteEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.remoteAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.remoteAuthToken)
	}
	resp, err := s.remoteClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("remote ATH signing failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("remote ATH signing returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode remote ATH signature: %w", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(result.Signature)
	if err != nil {
		return "", fmt.Errorf("invalid remote ATH signature encoding")
	}
	publicKey, err := parseECPublicKey(s.publicPEM)
	if err != nil || !ecdsa.VerifyASN1(publicKey, digest, signature) {
		return "", fmt.Errorf("remote ATH signature verification failed")
	}
	return result.Signature, nil
}

func (s *ServerSigner) KeyID() string {
	return s.keyID
}

func (s *ServerSigner) PublicKeyPEM() string {
	return s.publicPEM
}

func (s *ServerSigner) VerificationMethods() []VerificationMethod {
	methods := make([]VerificationMethod, len(s.verificationMethods))
	copy(methods, s.verificationMethods)
	return methods
}

type HandshakeService struct {
	store       HandshakeStore
	signer      *ServerSigner
	now         func() time.Time
	ttl         time.Duration
	sessionTTL  time.Duration
	replayStore RequestReplayStore
}

func NewHandshakeService(store HandshakeStore, signer *ServerSigner, ttl time.Duration) *HandshakeService {
	if ttl <= 0 {
		ttl = defaultHandshakeTTL
	}
	service := &HandshakeService{
		store: store, signer: signer, now: time.Now,
		ttl: ttl, sessionTTL: defaultSessionTTL,
	}
	if redisStore, ok := store.(*RedisHandshakeStore); ok {
		service.replayStore = NewRedisRequestReplayStore(redisStore.client)
	}
	return service
}

func (s *HandshakeService) SetSessionTTL(ttl time.Duration) {
	if ttl > 0 {
		s.sessionTTL = ttl
	}
}

func (s *HandshakeService) Start(ctx context.Context, input StartHandshakeInput) (*Handshake, error) {
	if s == nil || s.store == nil || s.signer == nil {
		return nil, fmt.Errorf("ATH handshake service is not configured")
	}
	now := s.now()
	if err := validateHandshakeTimestamp(now, input.Timestamp); err != nil {
		return nil, err
	}
	nonce, err := base64.RawURLEncoding.DecodeString(input.Nonce)
	if err != nil || len(nonce) < 32 {
		return nil, fmt.Errorf("client nonce must be base64url with at least 256 bits")
	}
	version := negotiateOne([]string{handshakeVersion}, input.Versions)
	if version == "" {
		return nil, fmt.Errorf("no supported ATH version")
	}
	capabilities := negotiateMany([]string{
		"ES256", "SHA-256", "OAuth2", "PKCE-S256",
		"ECDH-P256", "HKDF-SHA256", "HMAC-SHA256",
	}, input.Capabilities)
	for _, required := range []string{"ECDH-P256", "HKDF-SHA256", "HMAC-SHA256"} {
		if !containsString(capabilities, required) {
			return nil, fmt.Errorf("required handshake capability %s is missing", required)
		}
	}
	clientEphemeralBytes, err := base64.RawURLEncoding.DecodeString(input.EphemeralKey)
	if err != nil {
		return nil, fmt.Errorf("invalid client ephemeral key encoding")
	}
	clientEphemeralKey, err := ecdh.P256().NewPublicKey(clientEphemeralBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid client P-256 ephemeral key: %w", err)
	}
	serverEphemeralPrivate, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server ephemeral key: %w", err)
	}
	sharedSecret, err := serverEphemeralPrivate.ECDH(clientEphemeralKey)
	if err != nil {
		return nil, fmt.Errorf("derive ECDH shared secret: %w", err)
	}
	handshake := &Handshake{
		ID:                 GenerateSessionID(),
		ClientID:           input.ClientID,
		ClientDID:          input.ClientDID,
		ServerDID:          s.signer.did,
		Version:            version,
		Capabilities:       capabilities,
		ClientNonce:        input.Nonce,
		ServerNonce:        GenerateSessionID(),
		ClientEphemeralKey: input.EphemeralKey,
		ServerEphemeralKey: base64.RawURLEncoding.EncodeToString(serverEphemeralPrivate.PublicKey().Bytes()),
		ServerKeyID:        s.signer.keyID,
		ServerPublicKeyPEM: s.signer.publicPEM,
		State:              HandshakeStateChallengeIssued,
		CreatedAt:          now.Unix(),
		ExpiresAt:          now.Add(s.ttl).Unix(),
	}
	sessionKey, err := deriveSessionKey(sharedSecret, handshake)
	if err != nil {
		return nil, err
	}
	handshake.SessionKey = base64.RawURLEncoding.EncodeToString(sessionKey)
	payload := ServerChallengePayload{
		Type: serverChallengeType, HandshakeID: handshake.ID,
		ClientDID: handshake.ClientDID, ServerDID: handshake.ServerDID,
		ClientNonce: handshake.ClientNonce, ServerNonce: handshake.ServerNonce,
		ClientEphemeralKey: handshake.ClientEphemeralKey,
		ServerEphemeralKey: handshake.ServerEphemeralKey,
		Version:            handshake.Version, Timestamp: handshake.CreatedAt,
	}
	handshake.ServerSignature, err = s.signer.Sign(payload)
	if err != nil {
		return nil, err
	}
	if err := s.store.Create(ctx, handshake, s.ttl); err != nil {
		return nil, err
	}
	return handshake, nil
}

func (s *HandshakeService) Complete(ctx context.Context, input CompleteHandshakeInput) (*Handshake, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ATH handshake service is not configured")
	}
	now := s.now()
	if err := validateHandshakeTimestamp(now, input.Timestamp); err != nil {
		return nil, err
	}
	handshake, err := s.store.Get(ctx, input.HandshakeID)
	if err != nil {
		return nil, err
	}
	if handshake.State != HandshakeStateChallengeIssued {
		return nil, ErrHandshakeStateConflict
	}
	if now.Unix() > handshake.ExpiresAt {
		return nil, ErrHandshakeNotFound
	}
	if handshake.ClientID != input.ClientID || handshake.ClientDID != input.ClientDID {
		return nil, fmt.Errorf("handshake client identity mismatch")
	}
	publicKey, err := parseECPublicKey(input.PublicKeyPEM)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(input.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid client proof signature encoding")
	}
	payload := ClientIdentityProofPayload{
		Type: clientIdentityProofType, HandshakeID: handshake.ID,
		ClientDID: handshake.ClientDID, ServerDID: handshake.ServerDID,
		ServerNonce: handshake.ServerNonce, Version: handshake.Version,
		ClientEphemeralKey: handshake.ClientEphemeralKey,
		ServerEphemeralKey: handshake.ServerEphemeralKey,
		Timestamp:          input.Timestamp,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(publicKey, digest[:], signature) {
		return nil, fmt.Errorf("client identity proof signature verification failed")
	}
	handshake.State = HandshakeStateIdentityVerified
	handshake.VerifiedAt = now.Unix()
	handshake.ExpiresAt = now.Add(s.sessionTTL).Unix()
	if err := s.store.Transition(ctx, handshake, HandshakeStateChallengeIssued, s.sessionTTL); err != nil {
		return nil, err
	}
	return handshake, nil
}

func (s *HandshakeService) Get(ctx context.Context, id string) (*Handshake, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ATH handshake service is not configured")
	}
	handshake, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.now().Unix() > handshake.ExpiresAt {
		return nil, ErrHandshakeNotFound
	}
	return handshake, nil
}

func (s *HandshakeService) RequireVerified(ctx context.Context, id, clientDID, clientID string) (*Handshake, error) {
	handshake, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if handshake.ClientDID != clientDID || handshake.ClientID != clientID {
		return nil, fmt.Errorf("handshake client identity mismatch")
	}
	if handshake.State != HandshakeStateIdentityVerified {
		return nil, fmt.Errorf("handshake identity is not verified")
	}
	return handshake, nil
}

func (s *HandshakeService) ServerIdentityDocument() (*IdentityDocument, error) {
	if s == nil || s.signer == nil {
		return nil, fmt.Errorf("ATH handshake service is not configured")
	}
	return &IdentityDocument{
		ID:                 s.signer.did,
		VerificationMethod: s.signer.VerificationMethods(),
	}, nil
}

func validateHandshakeTimestamp(now time.Time, timestamp int64) error {
	if timestamp == 0 {
		return fmt.Errorf("timestamp is required")
	}
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < -time.Minute || delta > handshakeFreshness {
		return fmt.Errorf("handshake timestamp is not fresh")
	}
	return nil
}

func negotiateOne(supported, requested []string) string {
	for _, candidate := range requested {
		for _, value := range supported {
			if candidate == value {
				return candidate
			}
		}
	}
	return ""
}

func negotiateMany(supported, requested []string) []string {
	var result []string
	for _, candidate := range requested {
		if negotiateOne(supported, []string{candidate}) != "" {
			result = appendUniqueString(result, candidate)
		}
	}
	return result
}

func appendUniqueString(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func deriveSessionKey(sharedSecret []byte, handshake *Handshake) ([]byte, error) {
	clientNonce, err := base64.RawURLEncoding.DecodeString(handshake.ClientNonce)
	if err != nil {
		return nil, fmt.Errorf("decode client nonce: %w", err)
	}
	serverNonce, err := base64.RawURLEncoding.DecodeString(handshake.ServerNonce)
	if err != nil {
		return nil, fmt.Errorf("decode server nonce: %w", err)
	}
	saltInput := append(append([]byte{}, clientNonce...), serverNonce...)
	salt := sha256.Sum256(saltInput)
	info, err := json.Marshal(struct {
		Protocol    string `json:"protocol"`
		HandshakeID string `json:"handshake_id"`
		ClientDID   string `json:"client_did"`
		ServerDID   string `json:"server_did"`
		Version     string `json:"version"`
	}{
		Protocol:    "ATH-ECDH-P256-HKDF-SHA256",
		HandshakeID: handshake.ID, ClientDID: handshake.ClientDID,
		ServerDID: handshake.ServerDID, Version: handshake.Version,
	})
	if err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, sharedSecret, salt[:], info), key); err != nil {
		return nil, fmt.Errorf("derive ATH session key: %w", err)
	}
	return key, nil
}

var (
	defaultHandshakeServiceMu sync.RWMutex
	defaultHandshakeService   *HandshakeService
)

func SetDefaultHandshakeService(service *HandshakeService) {
	defaultHandshakeServiceMu.Lock()
	defer defaultHandshakeServiceMu.Unlock()
	defaultHandshakeService = service
}

func DefaultHandshakeService() *HandshakeService {
	defaultHandshakeServiceMu.RLock()
	defer defaultHandshakeServiceMu.RUnlock()
	return defaultHandshakeService
}
