package ath

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxIdentityDocumentSize = 1 << 20

// IdentityDocument is the subset of ATH and DID identity documents needed to
// authenticate an agent. PublicKey supports a PEM string or an EC JWK object.
type IdentityDocument struct {
	ATHVersion         string               `json:"ath_version,omitempty"`
	ID                 string               `json:"id,omitempty"`
	AgentID            string               `json:"agent_id,omitempty"`
	Name               string               `json:"name,omitempty"`
	Developer          *IdentityDeveloper   `json:"developer,omitempty"`
	PublicKey          json.RawMessage      `json:"public_key,omitempty"`
	VerificationMethod []VerificationMethod `json:"verificationMethod,omitempty"`
}

type IdentityDeveloper struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Contact string `json:"contact,omitempty"`
}

type VerificationMethod struct {
	ID           string          `json:"id"`
	Type         string          `json:"type,omitempty"`
	Controller   string          `json:"controller,omitempty"`
	PublicKeyJWK json.RawMessage `json:"publicKeyJwk,omitempty"`
	PublicKeyPEM string          `json:"publicKeyPem,omitempty"`
}

// IdentityResolver resolves an agent identifier to its identity document.
type IdentityResolver interface {
	Resolve(ctx context.Context, agentID string) (*IdentityDocument, error)
}

// HTTPIdentityResolver supports HTTPS identity documents and did:web.
type HTTPIdentityResolver struct {
	Client *http.Client
}

func NewHTTPIdentityResolver() *HTTPIdentityResolver {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid identity document address: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve identity document host: %w", err)
		}
		for _, ip := range ips {
			if !isPrivateIP(ip) {
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
		}
		return nil, errors.New("identity document host resolved only to private addresses")
	}
	return &HTTPIdentityResolver{
		Client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("too many identity document redirects")
				}
				return validatePublicIdentityURL(req.URL)
			},
		},
	}
}

func (r *HTTPIdentityResolver) Resolve(ctx context.Context, agentID string) (*IdentityDocument, error) {
	documentURL, err := identityDocumentURL(agentID)
	if err != nil {
		return nil, err
	}
	if err := validatePublicIdentityURL(documentURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, documentURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build identity document request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch identity document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity document returned HTTP %d", resp.StatusCode)
	}

	var document IdentityDocument
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxIdentityDocumentSize))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode identity document: %w", err)
	}
	if err := document.Validate(agentID); err != nil {
		return nil, err
	}
	return &document, nil
}

func (d *IdentityDocument) CanonicalID() string {
	if d.AgentID != "" {
		return d.AgentID
	}
	return d.ID
}

func (d *IdentityDocument) Validate(expectedAgentID string) error {
	if d.CanonicalID() != expectedAgentID {
		return fmt.Errorf("identity document id mismatch")
	}
	if _, _, err := d.VerificationKey(""); err != nil {
		return err
	}
	return nil
}

// VerificationKey selects kid when supplied, otherwise the document's direct
// public_key or its first verification method.
func (d *IdentityDocument) VerificationKey(kid string) (*ecdsa.PublicKey, string, error) {
	if len(d.PublicKey) > 0 {
		key, err := parseIdentityPublicKey(d.PublicKey)
		if err != nil {
			return nil, "", err
		}
		return key, kid, nil
	}

	for _, method := range d.VerificationMethod {
		if kid != "" && method.ID != kid {
			continue
		}
		if method.Controller != "" && method.Controller != d.CanonicalID() {
			continue
		}
		var raw json.RawMessage
		if len(method.PublicKeyJWK) > 0 {
			raw = method.PublicKeyJWK
		} else if method.PublicKeyPEM != "" {
			raw, _ = json.Marshal(method.PublicKeyPEM)
		} else {
			continue
		}
		key, err := parseIdentityPublicKey(raw)
		if err != nil {
			return nil, "", err
		}
		return key, method.ID, nil
	}
	if kid != "" {
		return nil, "", fmt.Errorf("identity document does not contain kid %q", kid)
	}
	return nil, "", errors.New("identity document does not contain an ES256 verification key")
}

func parseIdentityPublicKey(raw json.RawMessage) (*ecdsa.PublicKey, error) {
	var pemKey string
	if err := json.Unmarshal(raw, &pemKey); err == nil {
		return parseECPublicKey(pemKey)
	}

	var jwk struct {
		Kty string `json:"kty"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	}
	if err := json.Unmarshal(raw, &jwk); err != nil {
		return nil, fmt.Errorf("invalid identity public key: %w", err)
	}
	if jwk.Kty != "EC" || jwk.Crv != "P-256" || jwk.X == "" || jwk.Y == "" {
		return nil, errors.New("identity public key must be an EC P-256 JWK")
	}
	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("decode JWK x: %w", err)
	}
	y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("decode JWK y: %w", err)
	}
	key := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}
	if !key.Curve.IsOnCurve(key.X, key.Y) {
		return nil, errors.New("identity JWK point is not on P-256")
	}
	return key, nil
}

func identityDocumentURL(agentID string) (*url.URL, error) {
	if strings.HasPrefix(agentID, "did:web:") {
		methodID := strings.TrimPrefix(agentID, "did:web:")
		if methodID == "" {
			return nil, errors.New("empty did:web identifier")
		}
		parts := strings.Split(methodID, ":")
		host, err := url.PathUnescape(parts[0])
		if err != nil || host == "" {
			return nil, errors.New("invalid did:web host")
		}
		path := "/.well-known/did.json"
		if len(parts) > 1 {
			escaped := make([]string, 0, len(parts)-1)
			for _, part := range parts[1:] {
				decoded, decodeErr := url.PathUnescape(part)
				if decodeErr != nil || decoded == "" || strings.Contains(decoded, "/") {
					return nil, errors.New("invalid did:web path")
				}
				escaped = append(escaped, url.PathEscape(decoded))
			}
			path = "/" + strings.Join(escaped, "/") + "/did.json"
		}
		return &url.URL{Scheme: "https", Host: host, Path: path}, nil
	}

	u, err := url.Parse(agentID)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("agent_id must be a did:web or HTTPS identity document URI")
	}
	return u, nil
}

func validatePublicIdentityURL(u *url.URL) error {
	if u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return errors.New("identity document must use a public HTTPS URL")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("identity document host is not public")
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return errors.New("identity document IP is not public")
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
