package ath

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type staticIdentityResolver struct {
	document *IdentityDocument
	err      error
}

func (r staticIdentityResolver) Resolve(context.Context, string) (*IdentityDocument, error) {
	return r.document, r.err
}

func TestIdentityDocumentURL(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		want    string
		wantErr bool
	}{
		{name: "did web root", agentID: "did:web:agent.example.com", want: "https://agent.example.com/.well-known/did.json"},
		{name: "did web path", agentID: "did:web:example.com:agents:travel", want: "https://example.com/agents/travel/did.json"},
		{name: "https document", agentID: "https://agent.example.com/.well-known/agent.json", want: "https://agent.example.com/.well-known/agent.json"},
		{name: "unsupported did", agentID: "did:key:zExample", wantErr: true},
		{name: "insecure URL", agentID: "http://agent.example.com/agent.json", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := identityDocumentURL(tt.agentID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got.String())
		})
	}
}

func TestValidatePublicIdentityURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://localhost/agent.json",
		"https://127.0.0.1/agent.json",
		"https://10.0.0.1/agent.json",
		"https://169.254.169.254/latest/meta-data",
	} {
		u, err := identityDocumentURL(rawURL)
		require.NoError(t, err)
		require.Error(t, validatePublicIdentityURL(u), rawURL)
	}
}

func TestIdentityDocumentVerificationKeyJWK(t *testing.T) {
	privateKey := mustECDSAKey(t)
	jwk := map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(privateKey.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(privateKey.Y.Bytes()),
	}
	raw, err := json.Marshal(jwk)
	require.NoError(t, err)
	document := &IdentityDocument{
		AgentID:   "https://agent.example.com/.well-known/agent.json",
		PublicKey: raw,
	}
	key, _, err := document.VerificationKey("")
	require.NoError(t, err)
	require.Equal(t, privateKey.X, key.X)
	require.Equal(t, privateKey.Y, key.Y)
}

func TestAttestationVerifierRegistration(t *testing.T) {
	privateKey := mustECDSAKey(t)
	agentID := "https://agent.example.com/.well-known/agent.json"
	audience := "https://gateway.example.com/api/v1/ath/agents/register"
	document := identityDocumentWithPEM(t, agentID, &privateKey.PublicKey)
	now := time.Now().UTC().Truncate(time.Second)
	verifier := NewAttestationVerifier(staticIdentityResolver{document: document})
	verifier.Now = func() time.Time { return now }

	token := signedAttestation(t, privateKey, agentID, audience, now, jwt.SigningMethodES256)
	verified, err := verifier.VerifyRegistration(context.Background(), token, agentID, audience)
	require.NoError(t, err)
	require.Equal(t, agentID, verified.Claims.Subject)
	require.Contains(t, verified.PublicKeyPEM, "BEGIN PUBLIC KEY")
}

func TestAttestationVerifierRejectsInvalidClaims(t *testing.T) {
	privateKey := mustECDSAKey(t)
	agentID := "https://agent.example.com/.well-known/agent.json"
	audience := "https://gateway.example.com/api/v1/ath/token"
	now := time.Now().UTC().Truncate(time.Second)
	document := identityDocumentWithPEM(t, agentID, &privateKey.PublicKey)
	verifier := NewAttestationVerifier(staticIdentityResolver{document: document})
	verifier.Now = func() time.Time { return now }

	tests := []struct {
		name      string
		subject   string
		audience  string
		issuedAt  time.Time
		expiresAt time.Time
	}{
		{name: "wrong subject", subject: "https://evil.example/agent.json", audience: audience, issuedAt: now, expiresAt: now.Add(5 * time.Minute)},
		{name: "wrong audience", subject: agentID, audience: "https://gateway.example.com/wrong", issuedAt: now, expiresAt: now.Add(5 * time.Minute)},
		{name: "stale", subject: agentID, audience: audience, issuedAt: now.Add(-10 * time.Minute), expiresAt: now.Add(5 * time.Minute)},
		{name: "expired", subject: agentID, audience: audience, issuedAt: now.Add(-2 * time.Minute), expiresAt: now.Add(-time.Minute)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := AttestationClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    tt.subject,
					Subject:   tt.subject,
					Audience:  jwt.ClaimStrings{tt.audience},
					IssuedAt:  jwt.NewNumericDate(tt.issuedAt),
					ExpiresAt: jwt.NewNumericDate(tt.expiresAt),
					ID:        "jti-" + tt.name,
				},
			}
			token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
			signed, err := token.SignedString(privateKey)
			require.NoError(t, err)
			_, err = verifier.VerifyRegistration(context.Background(), signed, agentID, audience)
			require.Error(t, err)
		})
	}
}

func TestAttestationVerifierRejectsAlgorithmDowngrade(t *testing.T) {
	agentID := "https://agent.example.com/.well-known/agent.json"
	verifier := NewAttestationVerifier(staticIdentityResolver{err: errors.New("must not resolve")})
	claims := jwt.MapClaims{"iss": agentID, "sub": agentID, "aud": "gateway"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("not-an-es256-key"))
	require.NoError(t, err)
	_, err = verifier.VerifyRegistration(context.Background(), signed, agentID, "gateway")
	require.ErrorContains(t, err, "ES256")
}

func TestAttestationVerifierRejectsWrongSignature(t *testing.T) {
	trustedKey := mustECDSAKey(t)
	attackerKey := mustECDSAKey(t)
	agentID := "https://agent.example.com/.well-known/agent.json"
	audience := "https://gateway.example.com/api/v1/ath/token"
	now := time.Now().UTC().Truncate(time.Second)
	document := identityDocumentWithPEM(t, agentID, &trustedKey.PublicKey)
	verifier := NewAttestationVerifier(staticIdentityResolver{document: document})
	verifier.Now = func() time.Time { return now }

	token := signedAttestation(t, attackerKey, agentID, audience, now, jwt.SigningMethodES256)
	_, err := verifier.VerifyRegistration(context.Background(), token, agentID, audience)
	require.Error(t, err)
}

func TestIdentityDocumentRejectsMismatchedID(t *testing.T) {
	privateKey := mustECDSAKey(t)
	document := identityDocumentWithPEM(t, "https://other.example/agent.json", &privateKey.PublicKey)
	require.Error(t, document.Validate("https://agent.example.com/agent.json"))
}

func mustECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func identityDocumentWithPEM(t *testing.T, agentID string, key *ecdsa.PublicKey) *IdentityDocument {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	require.NoError(t, err)
	raw, err := json.Marshal(string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))
	require.NoError(t, err)
	return &IdentityDocument{ATHVersion: "0.1", AgentID: agentID, PublicKey: raw}
}

func signedAttestation(t *testing.T, key *ecdsa.PrivateKey, agentID, audience string, now time.Time, method jwt.SigningMethod) string {
	t.Helper()
	claims := AttestationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    agentID,
			Subject:   agentID,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			ID:        "jti-success",
		},
	}
	token := jwt.NewWithClaims(method, claims)
	token.Header["kid"] = agentID + "#key-1"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}
