package ath

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"agent-base/services/user-service/internal/model"
)

type memoryAuditStore struct {
	mu      sync.Mutex
	records []*model.ATHAuditRecord
}

func (s *memoryAuditStore) Append(_ context.Context, builder func(uint64, string) (*model.ATHAuditRecord, error)) (*model.ATHAuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sequence := uint64(len(s.records) + 1)
	previousHash := ""
	if len(s.records) > 0 {
		previousHash = s.records[len(s.records)-1].RecordHash
	}
	record, err := builder(sequence, previousHash)
	if err != nil {
		return nil, err
	}
	copy := *record
	s.records = append(s.records, &copy)
	return record, nil
}

func (s *memoryAuditStore) ListByHandshake(_ context.Context, handshakeID string, limit int) ([]*model.ATHAuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var records []*model.ATHAuditRecord
	for _, record := range s.records {
		if record.HandshakeID == handshakeID {
			copy := *record
			records = append(records, &copy)
		}
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	return records, nil
}

func (s *memoryAuditStore) ListAll(_ context.Context, _ int) ([]*model.ATHAuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]*model.ATHAuditRecord, 0, len(s.records))
	for _, record := range s.records {
		copy := *record
		records = append(records, &copy)
	}
	return records, nil
}

func (s *memoryAuditStore) Latest(_ context.Context) (*model.ATHAuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *s.records[len(s.records)-1]
	return &copy, nil
}

func newAuditTestService(t *testing.T) (*AuditService, *memoryAuditStore) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	signer, err := NewServerSigner("did:web:gateway.example.com", "", key)
	require.NoError(t, err)
	store := &memoryAuditStore{}
	service := NewAuditService(store, signer)
	service.now = func() time.Time { return time.Unix(1_750_000_000, 0) }
	return service, store
}

func TestAuditHashChainAndSignatures(t *testing.T) {
	service, _ := newAuditTestService(t)
	ctx := context.Background()
	first, err := service.Append(ctx, AuditEvent{
		EventType: AuditEventHandshakeStarted,
		ClientID:  "client-1", AgentID: "did:web:agent.example.com",
		HandshakeID: "handshake-1",
		Payload:     map[string]interface{}{"version": "0.1", "expires_at": 1_750_000_600},
	})
	require.NoError(t, err)
	second, err := service.Append(ctx, AuditEvent{
		EventType: AuditEventHandshakeVerified,
		ClientID:  "client-1", AgentID: "did:web:agent.example.com",
		HandshakeID: "handshake-1",
		Payload:     map[string]interface{}{"verified": true},
	})
	require.NoError(t, err)
	require.Equal(t, first.RecordHash, second.PreviousHash)

	verification, err := service.Verify(ctx)
	require.NoError(t, err)
	require.True(t, verification.Valid)
	require.Equal(t, 2, verification.RecordCount)
	require.Equal(t, second.RecordHash, verification.HeadHash)

	head, err := service.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, second.RecordHash, head.RecordHash)
	require.NotEmpty(t, head.Signature)
}

func TestAuditVerificationDetectsPayloadAndLinkTampering(t *testing.T) {
	service, store := newAuditTestService(t)
	ctx := context.Background()
	for _, eventType := range []string{AuditEventHandshakeStarted, AuditEventHandshakeVerified} {
		_, err := service.Append(ctx, AuditEvent{
			EventType: eventType, HandshakeID: "handshake-1",
			Payload: map[string]string{"status": eventType},
		})
		require.NoError(t, err)
	}

	store.records[0].Payload = `{"status":"tampered"}`
	verification, err := service.Verify(ctx)
	require.NoError(t, err)
	require.False(t, verification.Valid)
	require.Equal(t, uint64(1), verification.FailureSequence)
	require.Equal(t, "payload hash mismatch", verification.FailureReason)

	service, store = newAuditTestService(t)
	for _, eventType := range []string{AuditEventHandshakeStarted, AuditEventHandshakeVerified} {
		_, err := service.Append(ctx, AuditEvent{
			EventType: eventType, HandshakeID: "handshake-1",
			Payload: map[string]string{"status": eventType},
		})
		require.NoError(t, err)
	}
	store.records[1].PreviousHash = "forged"
	verification, err = service.Verify(ctx)
	require.NoError(t, err)
	require.False(t, verification.Valid)
	require.Equal(t, uint64(2), verification.FailureSequence)
	require.Equal(t, "previous hash mismatch", verification.FailureReason)

	service, store = newAuditTestService(t)
	_, err = service.Append(ctx, AuditEvent{
		EventType: AuditEventTokenIssued, HandshakeID: "handshake-1",
		Payload: map[string]string{"scope": "user:read"},
	})
	require.NoError(t, err)
	store.records[0].Signature = "invalid"
	verification, err = service.Verify(ctx)
	require.NoError(t, err)
	require.False(t, verification.Valid)
	require.Equal(t, "signature verification failed", verification.FailureReason)
}

func TestAuditCanonicalPayloadIsDeterministic(t *testing.T) {
	first, err := canonicalJSON(map[string]interface{}{"b": 2, "a": 1})
	require.NoError(t, err)
	second, err := canonicalJSON(map[string]interface{}{"a": 1, "b": 2})
	require.NoError(t, err)
	require.Equal(t, string(first), string(second))
}
