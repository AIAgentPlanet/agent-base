package types

// ATHRegisterRequest agent registration request
type ATHRegisterRequest struct {
	AgentID            string               `json:"agent_id" binding:"required"`
	AgentAttestation   string               `json:"agent_attestation"`
	Attestation        string               `json:"attestation,omitempty"`
	Name               string               `json:"name,omitempty" binding:"omitempty,max=256"`
	Developer          *ATHDeveloperInfo    `json:"developer" binding:"required"`
	RequestedProviders []ATHProviderRequest `json:"requested_providers"`
	RedirectURIs       []string             `json:"redirect_uris,omitempty"`
	Providers          []ATHProviderRequest `json:"providers,omitempty"`
	Purpose            string               `json:"purpose,omitempty"`
}

func (r *ATHRegisterRequest) EffectiveAttestation() string {
	if r.AgentAttestation != "" {
		return r.AgentAttestation
	}
	return r.Attestation
}

func (r *ATHRegisterRequest) EffectiveProviders() []ATHProviderRequest {
	if len(r.RequestedProviders) > 0 {
		return r.RequestedProviders
	}
	return r.Providers
}

// ATHDeveloperInfo developer information
type ATHDeveloperInfo struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// ATHProviderRequest requested provider and scopes
type ATHProviderRequest struct {
	ProviderID string   `json:"provider_id" binding:"required"`
	Scopes     []string `json:"scopes" binding:"required,min=1"`
}

// ATHRegisterResponse agent registration response
type ATHRegisterResponse struct {
	ClientID          string                `json:"client_id"`
	ClientSecret      string                `json:"client_secret"`
	AgentStatus       string                `json:"agent_status"`
	ApprovedProviders []ATHProviderApproval `json:"approved_providers"`
	ApprovalExpires   string                `json:"approval_expires"`
}

// ATHProviderApproval approved provider and scopes
type ATHProviderApproval struct {
	ProviderID     string   `json:"provider_id"`
	ApprovedScopes []string `json:"approved_scopes"`
	DeniedScopes   []string `json:"denied_scopes"`
	DenialReason   string   `json:"denial_reason,omitempty"`
}

// ATHAgentStatusResponse agent status query response
type ATHAgentStatusResponse struct {
	ClientID          string                `json:"client_id"`
	AgentID           string                `json:"agent_id"`
	Name              string                `json:"name"`
	Status            string                `json:"status"`
	ApprovedProviders []ATHProviderApproval `json:"approved_providers"`
	ApprovalExpires   string                `json:"approval_expires"`
}

// ATHHandshakeStartRequest starts ATH mutual identity verification.
type ATHHandshakeStartRequest struct {
	ClientID     string   `json:"client_id" binding:"required"`
	ClientDID    string   `json:"client_did" binding:"required"`
	Versions     []string `json:"versions" binding:"required,min=1"`
	Capabilities []string `json:"capabilities" binding:"required,min=1"`
	Nonce        string   `json:"nonce" binding:"required"`
	EphemeralKey string   `json:"ephemeral_key" binding:"required"`
	Timestamp    int64    `json:"timestamp" binding:"required"`
}

// ATHHandshakeChallengeResponse is the signed server challenge.
type ATHHandshakeChallengeResponse struct {
	HandshakeID        string   `json:"handshake_id"`
	Status             string   `json:"status"`
	ServerDID          string   `json:"server_did"`
	ServerKeyID        string   `json:"server_key_id"`
	ServerPublicKey    string   `json:"server_public_key"`
	Version            string   `json:"version"`
	Capabilities       []string `json:"capabilities"`
	ClientNonce        string   `json:"client_nonce"`
	ServerNonce        string   `json:"server_nonce"`
	ClientEphemeralKey string   `json:"client_ephemeral_key"`
	ServerEphemeralKey string   `json:"server_ephemeral_key"`
	Signature          string   `json:"signature"`
	Timestamp          int64    `json:"timestamp"`
	ExpiresAt          int64    `json:"expires_at"`
}

// ATHHandshakeProofRequest completes client identity verification.
type ATHHandshakeProofRequest struct {
	ClientID  string `json:"client_id" binding:"required"`
	Signature string `json:"signature" binding:"required"`
	Timestamp int64  `json:"timestamp" binding:"required"`
}

// ATHHandshakeStatusResponse reports the current handshake state.
type ATHHandshakeStatusResponse struct {
	HandshakeID           string `json:"handshake_id"`
	ClientDID             string `json:"client_did"`
	ServerDID             string `json:"server_did"`
	Status                string `json:"status"`
	Version               string `json:"version"`
	CreatedAt             int64  `json:"created_at"`
	ExpiresAt             int64  `json:"expires_at"`
	VerifiedAt            int64  `json:"verified_at,omitempty"`
	SessionKeyEstablished bool   `json:"session_key_established"`
}

// ATHAuthorizeRequest authorization initiation request
type ATHAuthorizeRequest struct {
	ClientID            string   `json:"client_id" binding:"required"`
	HandshakeID         string   `json:"handshake_id" binding:"required"`
	AgentAttestation    string   `json:"agent_attestation"`
	ProviderID          string   `json:"provider_id" binding:"required"`
	Scopes              []string `json:"scopes" binding:"required,min=1"`
	State               string   `json:"state" binding:"required"`
	UserRedirectURI     string   `json:"user_redirect_uri,omitempty"`
	RedirectURI         string   `json:"redirect_uri,omitempty"`
	Resource            string   `json:"resource,omitempty"`
	CodeChallenge       string   `json:"code_challenge,omitempty"`
	CodeChallengeMethod string   `json:"code_challenge_method,omitempty"`
}

func (r *ATHAuthorizeRequest) EffectiveRedirectURI() string {
	if r.UserRedirectURI != "" {
		return r.UserRedirectURI
	}
	return r.RedirectURI
}

// ATHAuthorizeResponse authorization initiation response
type ATHAuthorizeResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	ATHSessionID     string `json:"ath_session_id"`
	ExpiresIn        int    `json:"expires_in,omitempty"`
}

// ATHTokenRequest token exchange request
type ATHTokenRequest struct {
	GrantType        string `json:"grant_type" binding:"required,oneof=authorization_code"`
	Code             string `json:"code,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	ClientID         string `json:"client_id" binding:"required"`
	ClientSecret     string `json:"client_secret" binding:"required"`
	AgentAttestation string `json:"agent_attestation"`
	ATHSessionID     string `json:"ath_session_id,omitempty"`
	RedirectURI      string `json:"redirect_uri,omitempty"`
}

// ATHTokenResponse token exchange response
type ATHTokenResponse struct {
	AccessToken       string               `json:"access_token"`
	TokenType         string               `json:"token_type"`
	ExpiresIn         int                  `json:"expires_in"`
	EffectiveScopes   []string             `json:"effective_scopes"`
	ProviderID        string               `json:"provider_id"`
	AgentID           string               `json:"agent_id"`
	HandshakeID       string               `json:"handshake_id"`
	ScopeIntersection ATHScopeIntersection `json:"scope_intersection"`
}

type ATHScopeIntersection struct {
	AgentApproved []string `json:"agent_approved"`
	UserConsented []string `json:"user_consented"`
	Requested     []string `json:"requested,omitempty"`
	Effective     []string `json:"effective"`
}

// ATHRevokeRequest token revocation request
type ATHRevokeRequest struct {
	Token         string `json:"token" binding:"required"`
	TokenTypeHint string `json:"token_type_hint,omitempty"`
	ClientID      string `json:"client_id" binding:"required"`
	ClientSecret  string `json:"client_secret" binding:"required"`
}

// ATHProxyRequest proxy API request
type ATHProxyRequest struct {
	Provider         string `json:"provider" binding:"required"`
	Method           string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"`
	Path             string `json:"path" binding:"required"`
	Body             string `json:"body,omitempty"`
	RequestTimestamp int64  `json:"request_timestamp" binding:"required"`
	RequestNonce     string `json:"request_nonce" binding:"required"`
	RequestSignature string `json:"request_signature" binding:"required"`
}

type ATHAuditQueryRequest struct {
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
	HandshakeID  string `json:"handshake_id" binding:"required"`
	Limit        int    `json:"limit,omitempty" binding:"omitempty,min=1,max=500"`
}

type ATHAuditVerifyRequest struct {
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
}

type ATHAnchorAdminRequest struct {
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
}
