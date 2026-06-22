package integrity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotConfigured = errors.New("ATH integrity verifier is not configured")
	ErrInvalid       = errors.New("invalid ATH request integrity")
	ErrReplay        = errors.New("ATH request replay detected")
)

type Config struct {
	Secret          string
	FreshnessWindow time.Duration
	Now             func() time.Time
}

type Verifier struct {
	secret    []byte
	freshness time.Duration
	now       func() time.Time
	nonces    map[string]time.Time
	mu        sync.Mutex
}

type Input struct {
	Method        string
	Path          string
	SessionID     string
	ParticipantID string
	Body          []byte
	Timestamp     string
	Nonce         string
	BodySHA256    string
	Signature     string
}

type Payload struct {
	Type          string `json:"type"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	SessionID     string `json:"session_id"`
	ParticipantID string `json:"participant_id"`
	BodySHA256    string `json:"body_sha256"`
	Timestamp     int64  `json:"timestamp"`
	Nonce         string `json:"nonce"`
}

func New(config Config) *Verifier {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	freshness := config.FreshnessWindow
	if freshness == 0 {
		freshness = 5 * time.Minute
	}
	return &Verifier{
		secret:    []byte(config.Secret),
		freshness: freshness,
		now:       now,
		nonces:    make(map[string]time.Time),
	}
}

func (v *Verifier) Configured() bool {
	return v != nil && len(v.secret) > 0
}

func (v *Verifier) Verify(_ context.Context, input Input) error {
	if !v.Configured() {
		return ErrNotConfigured
	}
	timestamp, err := strconv.ParseInt(input.Timestamp, 10, 64)
	if err != nil || timestamp == 0 {
		return fmt.Errorf("%w: timestamp required", ErrInvalid)
	}
	now := v.now()
	requestTime := time.Unix(timestamp, 0)
	if requestTime.After(now.Add(time.Minute)) || now.Sub(requestTime) > v.freshness {
		return fmt.Errorf("%w: timestamp not fresh", ErrInvalid)
	}
	if input.Nonce == "" || len(input.Nonce) < 16 {
		return fmt.Errorf("%w: nonce too short", ErrInvalid)
	}
	bodyDigest := sha256.Sum256(input.Body)
	bodySHA256 := hex.EncodeToString(bodyDigest[:])
	if !strings.EqualFold(bodySHA256, input.BodySHA256) {
		return fmt.Errorf("%w: body digest mismatch", ErrInvalid)
	}
	payload := Payload{
		Type: "agent_base_ath_request_integrity", Method: strings.ToUpper(input.Method),
		Path: input.Path, SessionID: input.SessionID, ParticipantID: input.ParticipantID,
		BodySHA256: bodySHA256, Timestamp: timestamp, Nonce: input.Nonce,
	}
	expected, err := v.Sign(payload)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(input.Signature)) {
		return fmt.Errorf("%w: signature mismatch", ErrInvalid)
	}
	return v.useNonce(input.SessionID, input.ParticipantID, input.Nonce, now.Add(v.freshness))
}

func (v *Verifier) Sign(payload Payload) (string, error) {
	if !v.Configured() {
		return "", ErrNotConfigured
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (v *Verifier) useNonce(sessionID, participantID, nonce string, expiresAt time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	for key, expires := range v.nonces {
		if !expires.After(now) {
			delete(v.nonces, key)
		}
	}
	key := sessionID + ":" + participantID + ":" + nonce
	if _, ok := v.nonces[key]; ok {
		return ErrReplay
	}
	v.nonces[key] = expiresAt
	return nil
}
