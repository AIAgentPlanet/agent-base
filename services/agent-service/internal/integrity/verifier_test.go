package integrity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func TestVerifier(t *testing.T) {
	now := time.Unix(1000, 0)
	verifier := New(Config{Secret: "secret", Now: func() time.Time { return now }})
	body := []byte(`{"hello":"world"}`)
	bodyDigest := sha256.Sum256(body)
	bodySHA := hex.EncodeToString(bodyDigest[:])
	payload := Payload{
		Type: "agent_base_ath_request_integrity", Method: "POST",
		Path: "/api/v1/sessions/ses_1/messages", SessionID: "ses_1", ParticipantID: "par_1",
		BodySHA256: bodySHA, Timestamp: now.Unix(), Nonce: "nonce-1234567890",
	}
	signature, err := verifier.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	input := Input{
		Method: "POST", Path: payload.Path, SessionID: "ses_1", ParticipantID: "par_1",
		Body: body, Timestamp: "1000", Nonce: payload.Nonce, BodySHA256: bodySHA, Signature: signature,
	}
	if err := verifier.Verify(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify(context.Background(), input); !errors.Is(err, ErrReplay) {
		t.Fatalf("expected replay error, got %v", err)
	}
}

func TestVerifierRejectsBodyMismatch(t *testing.T) {
	now := time.Unix(1000, 0)
	verifier := New(Config{Secret: "secret", Now: func() time.Time { return now }})
	err := verifier.Verify(context.Background(), Input{
		Method: "POST", Path: "/x", SessionID: "ses_1", ParticipantID: "par_1",
		Body: []byte(`{}`), Timestamp: "1000", Nonce: "nonce-1234567890",
		BodySHA256: "bad", Signature: "bad",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
