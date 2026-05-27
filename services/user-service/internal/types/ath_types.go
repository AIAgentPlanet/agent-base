package types

// ATHRegisterRequest agent registration request
type ATHRegisterRequest struct {
	AgentID       string              `json:"agent_id" binding:"required"`
	Attestation   string              `json:"attestation" binding:"required"`
	Name          string              `json:"name" binding:"required,max=256"`
	Developer     *ATHDeveloperInfo   `json:"developer,omitempty"`
	RedirectURIs  []string            `json:"redirect_uris,omitempty"`
	Providers     []ATHProviderRequest `json:"providers,omitempty"`
	Purpose       string              `json:"purpose,omitempty"`
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
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ClientID          string              `json:"client_id"`
		ClientSecret      string              `json:"client_secret"`
		AgentStatus       string              `json:"agent_status"`
		ApprovedProviders []ATHProviderApproval `json:"approved_providers"`
		ApprovalExpiresAt int                 `json:"approval_expires_at"`
	} `json:"data"`
}

// ATHProviderApproval approved provider and scopes
type ATHProviderApproval struct {
	ProviderID string   `json:"provider_id"`
	Scopes     []string `json:"scopes"`
}

// ATHAgentStatusResponse agent status query response
type ATHAgentStatusResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ClientID          string              `json:"client_id"`
		AgentID           string              `json:"agent_id"`
		Name              string              `json:"name"`
		Status            string              `json:"status"`
		ApprovedProviders []ATHProviderApproval `json:"approved_providers"`
		ApprovalExpiresAt int                 `json:"approval_expires_at"`
	} `json:"data"`
}

// ATHAuthorizeRequest authorization initiation request
type ATHAuthorizeRequest struct {
	ClientID        string   `json:"client_id" binding:"required"`
	ProviderID      string   `json:"provider_id" binding:"required"`
	Scopes          []string `json:"scopes" binding:"required,min=1"`
	State           string   `json:"state" binding:"required"`
	RedirectURI     string   `json:"redirect_uri,omitempty"`
	CodeChallenge   string   `json:"code_challenge,omitempty"`
	CodeChallengeMethod string `json:"code_challenge_method,omitempty"`
}

// ATHAuthorizeResponse authorization initiation response
type ATHAuthorizeResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AuthorizationURL string `json:"authorization_url"`
		ATHSessionID     string `json:"ath_session_id"`
		ExpiresIn        int    `json:"expires_in"`
	} `json:"data"`
}

// ATHTokenRequest token exchange request
type ATHTokenRequest struct {
	GrantType    string `json:"grant_type" binding:"required,oneof=authorization_code refresh_token"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
	ATHSessionID string `json:"ath_session_id,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// ATHTokenResponse token exchange response
type ATHTokenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	} `json:"data"`
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
	Provider string `json:"provider" binding:"required"`
	Method   string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"`
	Path     string `json:"path" binding:"required"`
	Body     string `json:"body,omitempty"`
}
