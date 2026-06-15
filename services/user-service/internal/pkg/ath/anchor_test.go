package ath

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"agent-base/services/user-service/internal/model"
)

type memoryAnchorStore struct {
	mu        sync.Mutex
	records   []*model.ATHAuditOutbox
	delivered []uint64
	failed    []uint64
}

func (s *memoryAnchorStore) ClaimBatch(_ context.Context, limit int, _ time.Duration) ([]*model.ATHAuditOutbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > len(s.records) {
		limit = len(s.records)
	}
	records := append([]*model.ATHAuditOutbox(nil), s.records[:limit]...)
	s.records = s.records[limit:]
	return records, nil
}

func (s *memoryAnchorStore) MarkDelivered(_ context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delivered = append(s.delivered, id)
	return nil
}

func (s *memoryAnchorStore) MarkFailed(_ context.Context, id uint64, _ string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, id)
	return nil
}

func (s *memoryAnchorStore) RetryFailed(_ context.Context) (int64, error) {
	return int64(len(s.failed)), nil
}

func (s *memoryAnchorStore) Status(_ context.Context) (map[string]int64, error) {
	return map[string]int64{"delivered": int64(len(s.delivered)), "failed": int64(len(s.failed))}, nil
}

type fakeAnchorClient struct {
	failIDs map[uint64]bool
}

func (c *fakeAnchorClient) Deliver(_ context.Context, record *model.ATHAuditOutbox) error {
	if c.failIDs[record.ID] {
		return errors.New("anchor unavailable")
	}
	return nil
}

func TestAnchorWorkerDeliversAndRetries(t *testing.T) {
	store := &memoryAnchorStore{records: []*model.ATHAuditOutbox{
		{ID: 1, EventID: "event-1"},
		{ID: 2, EventID: "event-2"},
	}}
	worker := NewAnchorWorker(store, &fakeAnchorClient{failIDs: map[uint64]bool{2: true}}, time.Second, 10)
	count, err := worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, []uint64{1}, store.delivered)
	require.Equal(t, []uint64{2}, store.failed)
	require.Equal(t, time.Minute, anchorRetryDelay(1))
	require.Equal(t, 128*time.Minute, anchorRetryDelay(20))
}

func TestHTTPAnchorClientRequiresHTTPSInProduction(t *testing.T) {
	_, err := NewHTTPAnchorClient("http://anchor.example.com/events", "", time.Second, false)
	require.ErrorContains(t, err, "HTTPS")
	_, err = NewHTTPAnchorClient("https://anchor.example.com/events", "", time.Second, false)
	require.NoError(t, err)
}
