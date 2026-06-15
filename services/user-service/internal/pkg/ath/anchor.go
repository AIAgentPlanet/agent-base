package ath

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"agent-base/services/user-service/internal/model"
)

type AnchorOutboxStore interface {
	ClaimBatch(ctx context.Context, limit int, lockTTL time.Duration) ([]*model.ATHAuditOutbox, error)
	MarkDelivered(ctx context.Context, id uint64) error
	MarkFailed(ctx context.Context, id uint64, message string, retryAt time.Time) error
	RetryFailed(ctx context.Context) (int64, error)
	Status(ctx context.Context) (map[string]int64, error)
}

type AnchorClient interface {
	Deliver(ctx context.Context, record *model.ATHAuditOutbox) error
}

type HTTPAnchorClient struct {
	endpoint  string
	authToken string
	client    *http.Client
}

func NewHTTPAnchorClient(endpoint, authToken string, timeout time.Duration, allowHTTP bool) (*HTTPAnchorClient, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid ATH anchor endpoint")
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		return nil, fmt.Errorf("ATH anchor endpoint must use HTTPS")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPAnchorClient{
		endpoint: endpoint, authToken: authToken,
		client: &http.Client{Timeout: timeout},
	}, nil
}

func (c *HTTPAnchorClient) Deliver(ctx context.Context, record *model.ATHAuditOutbox) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewBufferString(record.Payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", record.EventID)
	req.Header.Set("X-ATH-Sequence", fmt.Sprintf("%d", record.Sequence))
	req.Header.Set("X-ATH-Record-Hash", record.RecordHash)
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("anchor returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type AnchorWorker struct {
	store    AnchorOutboxStore
	client   AnchorClient
	interval time.Duration
	batch    int
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

func NewAnchorWorker(store AnchorOutboxStore, client AnchorClient, interval time.Duration, batch int) *AnchorWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if batch <= 0 {
		batch = 50
	}
	return &AnchorWorker{
		store: store, client: client, interval: interval, batch: batch,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

func (w *AnchorWorker) Start() {
	if w == nil || w.store == nil || w.client == nil {
		return
	}
	w.once.Do(func() { go w.run() })
}

func (w *AnchorWorker) Close() error {
	if w == nil || w.store == nil || w.client == nil {
		return nil
	}
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	select {
	case <-w.done:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("ATH anchor worker shutdown timeout")
	}
	return nil
}

func (w *AnchorWorker) ProcessOnce(ctx context.Context) (int, error) {
	records, err := w.store.ClaimBatch(ctx, w.batch, 2*w.interval)
	if err != nil {
		return 0, err
	}
	for _, record := range records {
		if err := w.client.Deliver(ctx, record); err != nil {
			_ = w.store.MarkFailed(ctx, record.ID, err.Error(), time.Now().Add(anchorRetryDelay(record.Attempts+1)))
			continue
		}
		if err := w.store.MarkDelivered(ctx, record.ID); err != nil {
			return 0, err
		}
	}
	return len(records), nil
}

func (w *AnchorWorker) Status(ctx context.Context) (map[string]int64, error) {
	if w == nil || w.store == nil {
		return nil, fmt.Errorf("ATH anchor worker is not configured")
	}
	return w.store.Status(ctx)
}

func (w *AnchorWorker) Configured() bool {
	return w != nil && w.client != nil
}

func (w *AnchorWorker) RetryFailed(ctx context.Context) (int64, error) {
	if w == nil || w.store == nil {
		return 0, fmt.Errorf("ATH anchor worker is not configured")
	}
	return w.store.RetryFailed(ctx)
}

func (w *AnchorWorker) run() {
	defer close(w.done)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), w.interval)
		_, _ = w.ProcessOnce(ctx)
		cancel()
		select {
		case <-ticker.C:
		case <-w.stop:
			return
		}
	}
}

func anchorRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<uint(attempt-1)) * time.Minute
}

var (
	defaultAnchorWorkerMu sync.RWMutex
	defaultAnchorWorker   *AnchorWorker
)

func SetDefaultAnchorWorker(worker *AnchorWorker) {
	defaultAnchorWorkerMu.Lock()
	defer defaultAnchorWorkerMu.Unlock()
	defaultAnchorWorker = worker
}

func DefaultAnchorWorker() *AnchorWorker {
	defaultAnchorWorkerMu.RLock()
	defer defaultAnchorWorkerMu.RUnlock()
	return defaultAnchorWorker
}
