package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/pkg/jwt"
)

const (
	codePrefix          = "oauth:code:"
	CodeTTL             = 5 * time.Minute
	revokeAccessPrefix  = "oauth:revoke:access:"
	revokeRefreshPrefix = "oauth:revoke:refresh:"
)

// AuthorizationCode authorization code data
type AuthorizationCode struct {
	Code                    string `json:"code"`
	ClientID                string `json:"client_id"`
	UserID                  uint64 `json:"user_id"`
	RedirectURI             string `json:"redirect_uri"`
	Scope                   string `json:"scope"`
	State                   string `json:"state"`
	PKCECodeChallenge       string `json:"pkce_code_challenge,omitempty"`
	PKCECodeChallengeMethod string `json:"pkce_code_challenge_method,omitempty"`
	ExpiresAt               int64  `json:"expires_at"`
}

// TokenResponse token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}

// GenerateCode generates a random authorization code
func GenerateCode() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// GenerateClientID generates a random client id
func GenerateClientID() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// GenerateClientSecret generates a random client secret
func GenerateClientSecret() string {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// SaveAuthorizationCode saves authorization code to redis
func SaveAuthorizationCode(ctx context.Context, code *AuthorizationCode) error {
	if database.GetRedisCli() == nil {
		return fmt.Errorf("redis is required for oauth")
	}
	data, err := json.Marshal(code)
	if err != nil {
		return err
	}
	return database.GetRedisCli().Set(ctx, codePrefix+code.Code, string(data), CodeTTL).Err()
}

// GetAuthorizationCode gets and deletes authorization code from redis
func GetAuthorizationCode(ctx context.Context, code string) (*AuthorizationCode, error) {
	if database.GetRedisCli() == nil {
		return nil, fmt.Errorf("redis is required for oauth")
	}
	key := codePrefix + code
	data, err := database.GetRedisCli().Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("authorization code not found or expired")
	}
	if err != nil {
		return nil, err
	}
	// delete after use (one-time)
	_ = database.GetRedisCli().Del(ctx, key).Err()

	var ac AuthorizationCode
	if err := json.Unmarshal([]byte(data), &ac); err != nil {
		return nil, err
	}
	if time.Now().Unix() > ac.ExpiresAt {
		return nil, fmt.Errorf("authorization code expired")
	}
	return &ac, nil
}

// GenerateTokenPair generates access token and refresh token
func GenerateTokenPair(userID uint64, clientID string, scope string) (*TokenResponse, error) {
	cfg := config.Get().OAuth
	accessExpire := 3600 // default 1 hour
	if cfg.AccessTokenExpire > 0 {
		accessExpire = cfg.AccessTokenExpire
	}
	refreshExpire := 2592000 // default 30 days
	if cfg.RefreshTokenExpire > 0 {
		refreshExpire = cfg.RefreshTokenExpire
	}

	accessToken, err := jwt.GenerateOAuthToken(userID, clientID, scope, "access", accessExpire)
	if err != nil {
		return nil, err
	}
	refreshToken, err := jwt.GenerateOAuthToken(userID, clientID, scope, "refresh", refreshExpire)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    accessExpire,
		Scope:        scope,
	}, nil
}

// ParseAndValidateToken parses and validates an oauth token
func ParseAndValidateToken(tokenString string) (*jwt.OAuthClaims, error) {
	claims, err := jwt.ParseOAuthToken(tokenString)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// RevokeToken revokes a token by adding it to blacklist
func RevokeToken(ctx context.Context, tokenString string, tokenType string) error {
	if database.GetRedisCli() == nil {
		return fmt.Errorf("redis is required for oauth")
	}
	claims, err := jwt.ParseOAuthToken(tokenString)
	if err != nil {
		return err
	}

	expiresAt := claims.ExpiresAt
	if expiresAt == nil || expiresAt.Time.IsZero() {
		return fmt.Errorf("token has no expiration")
	}
	ttl := time.Until(expiresAt.Time)
	if ttl <= 0 {
		return nil // already expired
	}

	var prefix string
	if tokenType == "refresh" {
		prefix = revokeRefreshPrefix
	} else {
		prefix = revokeAccessPrefix
	}

	jti := claims.ID
	if jti == "" {
		// fallback to token hash
		h := sha256.Sum256([]byte(tokenString))
		jti = base64.URLEncoding.EncodeToString(h[:16])
	}

	return database.GetRedisCli().Set(ctx, prefix+jti, "1", ttl).Err()
}

// IsTokenRevoked checks if a token is revoked
func IsTokenRevoked(ctx context.Context, tokenString string, tokenType string) (bool, error) {
	if database.GetRedisCli() == nil {
		return false, fmt.Errorf("redis is required for oauth")
	}
	claims, err := jwt.ParseOAuthToken(tokenString)
	if err != nil {
		return false, err
	}

	var prefix string
	if tokenType == "refresh" {
		prefix = revokeRefreshPrefix
	} else {
		prefix = revokeAccessPrefix
	}

	jti := claims.ID
	if jti == "" {
		h := sha256.Sum256([]byte(tokenString))
		jti = base64.URLEncoding.EncodeToString(h[:16])
	}

	exists, err := database.GetRedisCli().Exists(ctx, prefix+jti).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// ValidatePKCE validates PKCE code_verifier against code_challenge
func ValidatePKCE(verifier, challenge, method string) bool {
	if challenge == "" {
		return true // no PKCE used
	}
	if verifier == "" {
		return false
	}
	if method == "" || method == "plain" {
		return verifier == challenge
	}
	if method == "S256" {
		h := sha256.Sum256([]byte(verifier))
		encoded := base64.RawURLEncoding.EncodeToString(h[:])
		return encoded == challenge
	}
	return false
}

// ValidateRedirectURI validates redirect URI against client's allowed URIs
func ValidateRedirectURI(client *model.OAuthClient, redirectURI string) bool {
	if redirectURI == "" {
		return false
	}
	var uris []string
	if err := json.Unmarshal([]byte(client.RedirectURIs), &uris); err != nil {
		return false
	}
	for _, uri := range uris {
		if strings.TrimSpace(uri) == redirectURI {
			return true
		}
	}
	return false
}

// ValidateScope validates scope against client's allowed scopes
func ValidateScope(client *model.OAuthClient, scope string) bool {
	if scope == "" {
		return true
	}
	var allowedScopes []string
	if err := json.Unmarshal([]byte(client.AllowedScopes), &allowedScopes); err != nil {
		return false
	}
	allowedMap := make(map[string]bool)
	for _, s := range allowedScopes {
		allowedMap[strings.TrimSpace(s)] = true
	}
	requested := strings.Split(scope, " ")
	for _, s := range requested {
		if !allowedMap[strings.TrimSpace(s)] {
			return false
		}
	}
	return true
}

// ValidateGrantType validates grant type against client's allowed grants
func ValidateGrantType(client *model.OAuthClient, grantType string) bool {
	var allowedGrants []string
	if err := json.Unmarshal([]byte(client.AllowedGrants), &allowedGrants); err != nil {
		return false
	}
	for _, g := range allowedGrants {
		if strings.TrimSpace(g) == grantType {
			return true
		}
	}
	return false
}
