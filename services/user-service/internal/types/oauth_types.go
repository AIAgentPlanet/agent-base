package types

import (
	"github.com/go-dev-frame/sponge/pkg/sgorm/query"
)

// ==================== OAuth Client Types ====================

// CreateOAuthClientRequest request params
type CreateOAuthClientRequest struct {
	Name          string   `json:"name" binding:"required,max=256"`
	RedirectURIs  []string `json:"redirect_uris" binding:"required,min=1"`
	AllowedGrants []string `json:"allowed_grants" binding:"omitempty"`
	AllowedScopes []string `json:"allowed_scopes" binding:"omitempty"`
}

// CreateOAuthClientReply only for api docs
type CreateOAuthClientReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID           uint64 `json:"id"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"data"`
}

// OAuthClientObjDetail detail
type OAuthClientObjDetail struct {
	ID            uint64   `json:"id"`
	ClientID      string   `json:"clientId"`
	Name          string   `json:"name"`
	RedirectURIs  []string `json:"redirectUris"`
	AllowedGrants []string `json:"allowedGrants"`
	AllowedScopes []string `json:"allowedScopes"`
	Status        int      `json:"status"`
	CreatedAt     int      `json:"createdAt"`
	UpdatedAt     int      `json:"updatedAt"`
}

// UpdateOAuthClientRequest request params
type UpdateOAuthClientRequest struct {
	Name          string   `json:"name" binding:"omitempty,max=256"`
	RedirectURIs  []string `json:"redirect_uris" binding:"omitempty,min=1"`
	AllowedGrants []string `json:"allowed_grants" binding:"omitempty"`
	AllowedScopes []string `json:"allowed_scopes" binding:"omitempty"`
	Status        int      `json:"status" binding:"omitempty,oneof=1 2"`
}

// UpdateOAuthClientReply only for api docs
type UpdateOAuthClientReply struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data struct{} `json:"data"`
}

// GetOAuthClientByIDReply only for api docs
type GetOAuthClientByIDReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Client OAuthClientObjDetail `json:"client"`
	} `json:"data"`
}

// ListOAuthClientsRequest request params
type ListOAuthClientsRequest struct {
	query.Params
}

// ListOAuthClientsReply only for api docs
type ListOAuthClientsReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Clients []OAuthClientObjDetail `json:"clients"`
		Total   int64                  `json:"total"`
	} `json:"data"`
}

// DeleteOAuthClientByIDReply only for api docs
type DeleteOAuthClientByIDReply struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data struct{} `json:"data"`
}

// ==================== OAuth Authorization Types ====================

// OAuthAuthorizeRequest authorization endpoint request (query params)
type OAuthAuthorizeRequest struct {
	ResponseType            string `form:"response_type" binding:"required,oneof=code"`
	ClientID                string `form:"client_id" binding:"required"`
	RedirectURI             string `form:"redirect_uri" binding:"required,url"`
	Scope                   string `form:"scope" binding:"omitempty"`
	State                   string `form:"state" binding:"omitempty"`
	CodeChallenge           string `form:"code_challenge" binding:"omitempty"`
	CodeChallengeMethod     string `form:"code_challenge_method" binding:"omitempty,oneof=plain S256"`
}

// OAuthTokenRequest token endpoint request
type OAuthTokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type" binding:"required,oneof=authorization_code refresh_token"`
	Code         string `json:"code,omitempty" form:"code,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty" form:"redirect_uri,omitempty"`
	ClientID     string `json:"client_id" form:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" form:"client_secret" binding:"required"`
	RefreshToken string `json:"refresh_token,omitempty" form:"refresh_token,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty" form:"code_verifier,omitempty"`
}

// OAuthTokenReply token endpoint response
type OAuthTokenReply struct {
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

// OAuthUserInfoReply userinfo endpoint response
type OAuthUserInfoReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID       uint64 `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	} `json:"data"`
}

// OAuthRevokeRequest revoke endpoint request
type OAuthRevokeRequest struct {
	Token         string `json:"token" binding:"required"`
	TokenTypeHint string `json:"token_type_hint" binding:"omitempty,oneof=access_token refresh_token"`
}

// OAuthRevokeReply revoke endpoint response
type OAuthRevokeReply struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data struct{} `json:"data"`
}
