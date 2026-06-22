package athclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestVerifyPostsCredentials(t *testing.T) {
	var requestBody map[string]any
	client := New(Config{
		BaseURL:      "http://user-service.test",
		ClientID:     "client",
		ClientSecret: "secret",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v1/ath/audit/verify" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatal(err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(`{"valid":true}`)),
			}, nil
		})},
	})

	result, err := client.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requestBody["client_id"] != "client" || requestBody["client_secret"] != "secret" {
		t.Fatalf("credentials were not posted: %v", requestBody)
	}
	if result["valid"] != true {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestIntrospectPostsToken(t *testing.T) {
	var requestBody map[string]any
	client := New(Config{
		BaseURL:      "http://user-service.test",
		ClientID:     "client",
		ClientSecret: "secret",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v1/ath/introspect" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatal(err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(`{"active":true}`)),
			}, nil
		})},
	})

	result, err := client.Introspect(context.Background(), "token-1")
	if err != nil {
		t.Fatal(err)
	}
	if requestBody["token"] != "token-1" {
		t.Fatalf("token was not posted: %v", requestBody)
	}
	if result["active"] != true {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnconfiguredClient(t *testing.T) {
	client := New(Config{})
	if client.Configured() {
		t.Fatal("expected unconfigured client")
	}
	if _, err := client.Verify(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
