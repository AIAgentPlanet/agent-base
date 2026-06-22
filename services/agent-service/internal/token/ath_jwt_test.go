package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestATHJWTVerifier(t *testing.T) {
	now := time.Unix(1000, 0)
	token := signTestToken(t, "secret", map[string]any{
		"type":         "ath_access",
		"iss":          "user-service",
		"exp":          1300,
		"nbf":          900,
		"session_id":   "ses_1",
		"handshake_id": "hsk_1",
		"agent_id":     "https://agent.example/.well-known/agent.json",
		"scope":        "session:read session:speak",
	})
	verifier := NewATHJWTVerifier(Config{Secret: "secret", Issuer: "user-service", Now: func() time.Time { return now }})
	claims, err := verifier.Verify(VerifyInput{
		Token:         token,
		SessionID:     "ses_1",
		HandshakeID:   "hsk_1",
		AgentIdentity: "https://agent.example/.well-known/agent.json",
		RequiredScope: "session:speak",
	})
	if err != nil {
		t.Fatal(err)
	}
	if claims.SessionID != "ses_1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestATHJWTVerifierRejectsWrongScope(t *testing.T) {
	token := signTestToken(t, "secret", map[string]any{
		"type":       "ath_access",
		"exp":        1300,
		"session_id": "ses_1",
		"scope":      "session:read",
	})
	verifier := NewATHJWTVerifier(Config{Secret: "secret", Now: func() time.Time { return time.Unix(1000, 0) }})
	if _, err := verifier.Verify(VerifyInput{Token: token, SessionID: "ses_1", RequiredScope: "session:speak"}); err == nil {
		t.Fatal("expected scope error")
	}
}

func signTestToken(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	head := base64.RawURLEncoding.EncodeToString(headerJSON)
	body := base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(head + "." + body))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return head + "." + body + "." + signature
}
