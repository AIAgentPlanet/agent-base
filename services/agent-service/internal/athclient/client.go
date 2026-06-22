package athclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
	HTTPClient   *http.Client
}

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

func New(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: config.Timeout}
	}
	return &Client{
		baseURL:      strings.TrimRight(config.BaseURL, "/"),
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		httpClient:   httpClient,
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.clientID != "" && c.clientSecret != ""
}

func (c *Client) Verify(ctx context.Context) (map[string]any, error) {
	return c.post(ctx, "/api/v1/ath/audit/verify", map[string]any{
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
	})
}

func (c *Client) Query(ctx context.Context, handshakeID string, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	return c.post(ctx, "/api/v1/ath/audit/query", map[string]any{
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"handshake_id":  handshakeID,
		"limit":         limit,
	})
}

func (c *Client) Introspect(ctx context.Context, token string) (map[string]any, error) {
	return c.post(ctx, "/api/v1/ath/introspect", map[string]any{
		"client_id":       c.clientID,
		"client_secret":   c.clientSecret,
		"token":           token,
		"token_type_hint": "access_token",
	})
}

func (c *Client) post(ctx context.Context, path string, body map[string]any) (map[string]any, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("ATH client is not configured")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var decoded map[string]any
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("ATH %s returned %d: %v", path, response.StatusCode, decoded)
	}
	return decoded, nil
}
