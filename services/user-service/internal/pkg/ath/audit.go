package ath

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"agent-base/services/user-service/internal/model"
)

const (
	AuditEventHandshakeStarted     = "handshake.started"
	AuditEventHandshakeVerified    = "handshake.verified"
	AuditEventAuthorizationStarted = "authorization.started"
	AuditEventTokenIssued          = "token.issued"
	AuditEventTokenRefreshed       = "token.refreshed"
	AuditEventTokenRevoked         = "token.revoked"
	AuditEventProxyAllowed         = "proxy.allowed"
)

type AuditStore interface {
	Append(ctx context.Context, builder func(sequence uint64, previousHash string) (*model.ATHAuditRecord, error)) (*model.ATHAuditRecord, error)
	ListByHandshake(ctx context.Context, handshakeID string, limit int) ([]*model.ATHAuditRecord, error)
	ListAll(ctx context.Context, limit int) ([]*model.ATHAuditRecord, error)
	Latest(ctx context.Context) (*model.ATHAuditRecord, error)
}

type AuditEvent struct {
	EventType   string
	ClientID    string
	AgentID     string
	HandshakeID string
	TokenID     string
	Payload     interface{}
}

type AuditVerification struct {
	Valid           bool   `json:"valid"`
	RecordCount     int    `json:"record_count"`
	HeadSequence    uint64 `json:"head_sequence"`
	HeadHash        string `json:"head_hash"`
	FailureSequence uint64 `json:"failure_sequence,omitempty"`
	FailureReason   string `json:"failure_reason,omitempty"`
}

type AuditHead struct {
	Sequence     uint64 `json:"sequence"`
	RecordHash   string `json:"record_hash"`
	Signature    string `json:"signature"`
	SigningKeyID string `json:"signing_key_id"`
	Timestamp    int64  `json:"timestamp"`
}

type auditCanonicalRecord struct {
	Sequence         uint64 `json:"sequence"`
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	ClientID         string `json:"client_id,omitempty"`
	AgentID          string `json:"agent_id,omitempty"`
	HandshakeID      string `json:"handshake_id,omitempty"`
	TokenID          string `json:"token_id,omitempty"`
	PayloadHash      string `json:"payload_hash"`
	PreviousHash     string `json:"previous_hash"`
	SigningKeyID     string `json:"signing_key_id"`
	SigningPublicKey string `json:"signing_public_key"`
	CreatedAt        int64  `json:"created_at"`
}

type AuditService struct {
	store  AuditStore
	signer *ServerSigner
	now    func() time.Time
}

func NewAuditService(store AuditStore, signer *ServerSigner) *AuditService {
	return &AuditService{store: store, signer: signer, now: time.Now}
}

func (s *AuditService) Append(ctx context.Context, event AuditEvent) (*model.ATHAuditRecord, error) {
	if s == nil || s.store == nil || s.signer == nil {
		return nil, fmt.Errorf("ATH audit service is not configured")
	}
	if event.EventType == "" {
		return nil, fmt.Errorf("audit event type is required")
	}
	payload, err := canonicalJSON(event.Payload)
	if err != nil {
		return nil, err
	}
	payloadDigest := sha256.Sum256(payload)
	createdAt := s.now().Unix()
	return s.store.Append(ctx, func(sequence uint64, previousHash string) (*model.ATHAuditRecord, error) {
		record := &model.ATHAuditRecord{
			Sequence: sequence, EventID: GenerateSessionID(),
			EventType: event.EventType, ClientID: event.ClientID,
			AgentID: event.AgentID, HandshakeID: event.HandshakeID,
			TokenID: event.TokenID, Payload: string(payload),
			PayloadHash:  hex.EncodeToString(payloadDigest[:]),
			PreviousHash: previousHash, SigningKeyID: s.signer.KeyID(),
			SigningPublicKey: s.signer.PublicKeyPEM(), CreatedAt: createdAt,
		}
		digest, err := auditRecordDigest(record)
		if err != nil {
			return nil, err
		}
		record.RecordHash = hex.EncodeToString(digest)
		record.Signature, err = s.signer.SignDigest(digest)
		if err != nil {
			return nil, err
		}
		return record, nil
	})
}

func (s *AuditService) ListByHandshake(ctx context.Context, handshakeID string, limit int) ([]*model.ATHAuditRecord, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ATH audit service is not configured")
	}
	return s.store.ListByHandshake(ctx, handshakeID, limit)
}

func (s *AuditService) Verify(ctx context.Context) (*AuditVerification, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ATH audit service is not configured")
	}
	records, err := s.store.ListAll(ctx, 0)
	if err != nil {
		return nil, err
	}
	result := &AuditVerification{Valid: true, RecordCount: len(records)}
	previousHash := ""
	for index, record := range records {
		expectedSequence := uint64(index + 1)
		if record.Sequence != expectedSequence {
			return auditFailure(result, record.Sequence, "non-contiguous sequence"), nil
		}
		if record.PreviousHash != previousHash {
			return auditFailure(result, record.Sequence, "previous hash mismatch"), nil
		}
		payloadDigest := sha256.Sum256([]byte(record.Payload))
		if record.PayloadHash != hex.EncodeToString(payloadDigest[:]) {
			return auditFailure(result, record.Sequence, "payload hash mismatch"), nil
		}
		digest, err := auditRecordDigest(record)
		if err != nil {
			return nil, err
		}
		if record.RecordHash != hex.EncodeToString(digest) {
			return auditFailure(result, record.Sequence, "record hash mismatch"), nil
		}
		publicKey, err := parseECPublicKey(record.SigningPublicKey)
		if err != nil {
			return auditFailure(result, record.Sequence, "invalid signing public key"), nil
		}
		signature, err := base64.RawURLEncoding.DecodeString(record.Signature)
		if err != nil || !ecdsa.VerifyASN1(publicKey, digest, signature) {
			return auditFailure(result, record.Sequence, "signature verification failed"), nil
		}
		previousHash = record.RecordHash
		result.HeadSequence = record.Sequence
		result.HeadHash = record.RecordHash
	}
	return result, nil
}

func (s *AuditService) Head(ctx context.Context) (*AuditHead, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ATH audit service is not configured")
	}
	record, err := s.store.Latest(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &AuditHead{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &AuditHead{
		Sequence: record.Sequence, RecordHash: record.RecordHash,
		Signature: record.Signature, SigningKeyID: record.SigningKeyID,
		Timestamp: record.CreatedAt,
	}, nil
}

func auditRecordDigest(record *model.ATHAuditRecord) ([]byte, error) {
	canonical := auditCanonicalRecord{
		Sequence: record.Sequence, EventID: record.EventID,
		EventType: record.EventType, ClientID: record.ClientID,
		AgentID: record.AgentID, HandshakeID: record.HandshakeID,
		TokenID: record.TokenID, PayloadHash: record.PayloadHash,
		PreviousHash: record.PreviousHash, SigningKeyID: record.SigningKeyID,
		SigningPublicKey: record.SigningPublicKey, CreatedAt: record.CreatedAt,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	return digest[:], nil
}

func canonicalJSON(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal audit payload: %w", err)
	}
	var compact json.RawMessage
	if err := json.Unmarshal(data, &compact); err != nil {
		return nil, fmt.Errorf("normalize audit payload: %w", err)
	}
	return json.Marshal(compact)
}

func auditFailure(result *AuditVerification, sequence uint64, reason string) *AuditVerification {
	result.Valid = false
	result.FailureSequence = sequence
	result.FailureReason = reason
	return result
}

var (
	defaultAuditServiceMu sync.RWMutex
	defaultAuditService   *AuditService
)

func SetDefaultAuditService(service *AuditService) {
	defaultAuditServiceMu.Lock()
	defer defaultAuditServiceMu.Unlock()
	defaultAuditService = service
}

func DefaultAuditService() *AuditService {
	defaultAuditServiceMu.RLock()
	defer defaultAuditServiceMu.RUnlock()
	return defaultAuditService
}
