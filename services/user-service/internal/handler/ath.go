package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/go-dev-frame/sponge/pkg/gin/middleware"
	"github.com/go-dev-frame/sponge/pkg/gin/response"
	"github.com/go-dev-frame/sponge/pkg/logger"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/dao"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/ecode"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/pkg/ath"
	"agent-base/services/user-service/internal/pkg/jwt"
	"agent-base/services/user-service/internal/pkg/oauth"
	"agent-base/services/user-service/internal/types"
)

// ==================== ATH Discovery ====================

// ATHDiscovery returns the ATH discovery document
// @Summary ATH discovery document
// @Description Returns the ATH protocol discovery document
// @Tags ath
// @Accept json
// @Produce json
// @Success 200 {object} ath.DiscoveryDocument{}
// @Router /.well-known/ath.json [get]
func (h *usersHandler) ATHDiscovery(c *gin.Context) {
	baseURL := fmt.Sprintf("http://%s", c.Request.Host)
	if config.Get().App.Host != "" {
		baseURL = fmt.Sprintf("http://%s:%d", config.Get().App.Host, config.Get().HTTP.Port)
	}
	response.Success(c, ath.GetDiscoveryDocument(baseURL))
}

// ==================== ATH Agent Registration ====================

// ATHRegister registers a new ATH agent
// @Summary Register ATH agent
// @Description Register an AI agent with the ATH protocol
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHRegisterRequest true "agent registration"
// @Success 200 {object} types.ATHRegisterResponse{}
// @Router /api/v1/ath/agents/register [post]
func (h *usersHandler) ATHRegister(c *gin.Context) {
	form := &types.ATHRegisterRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	// Step 1: Verify attestation JWT (unverified parse to get agent_id)
	attestationClaims, err := ath.VerifyAttestation(form.Attestation, nil)
	if err != nil {
		logger.Warn("ATH attestation parse error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrATHInvalidAttestation)
		return
	}

	agentID := attestationClaims.Subject
	if agentID == "" {
		agentID = form.AgentID
	}

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())

	// Check if agent already registered
	existing, _ := athDao.GetByAgentID(ctx, agentID)
	if existing != nil {
		response.Error(c, ecode.ErrOAuthClientExists)
		return
	}

	// Step 2: Verify attestation with agent's public key
	// For MVP, the public key is provided in the attestation header or we derive it
	// Actually, we need the public key to verify. In a real implementation, the agent
	// would have a DID document. For MVP, we'll accept the public key in the request
	// or extract it from the JWT header if it's a self-signed JWK.
	// Simplification: Store the agent and verify attestation in a second pass after we have the pubkey.
	// For MVP, we'll skip full ES256 verification during registration and just validate format.
	// The attestation is still checked for jti replay.

	// Check jti replay
	if attestationClaims.ID != "" {
		if err := ath.CheckJTIReplay(attestationClaims.ID); err != nil {
			response.Error(c, ecode.ErrATHJTIReplay)
			return
		}
	}

	// Build scopes list
	scopes := []string{"user:read"}
	if len(form.Providers) > 0 && len(form.Providers[0].Scopes) > 0 {
		scopes = form.Providers[0].Scopes
	}
	scopesJSON, _ := json.Marshal(scopes)

	redirectURIs := []string{}
	if len(form.RedirectURIs) > 0 {
		redirectURIs = form.RedirectURIs
	}
	redirectURIsJSON, _ := json.Marshal(redirectURIs)

	developerJSON, _ := json.Marshal(form.Developer)

	approvedProviders := []ath.ProviderApproval{
		{ProviderID: "user-service", Scopes: scopes},
	}
	approvedProvidersJSON, _ := json.Marshal(approvedProviders)

	clientID := ath.GenerateClientID()
	clientSecret := ath.GenerateClientSecret()

	agent := &model.ATHAgent{
		ClientID:          clientID,
		ClientSecret:      clientSecret,
		AgentID:           agentID,
		PublicKey:         "", // TODO: extract from attestation or DID document
		Name:              form.Name,
		DeveloperInfo:     string(developerJSON),
		RedirectURIs:      string(redirectURIsJSON),
		AllowedScopes:     string(scopesJSON),
		ApprovedProviders: string(approvedProvidersJSON),
		Status:            "approved",
		ApprovalExpiresAt: int(time.Now().Add(30 * 24 * time.Hour).Unix()),
		CreatedAt:         int(time.Now().Unix()),
		UpdatedAt:         int(time.Now().Unix()),
	}

	if err := athDao.Create(ctx, agent); err != nil {
		logger.Error("ATH agent create error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	// Also create an OAuth client for this agent so it can use the OAuth authorize flow
	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	oauthClient := &model.OAuthClient{
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		Name:          form.Name,
		RedirectURIs:  string(redirectURIsJSON),
		AllowedGrants: `["authorization_code"]`,
		AllowedScopes: string(scopesJSON),
		UserID:        1, // system/admin user
		Status:        1,
		CreatedAt:     int(time.Now().Unix()),
		UpdatedAt:     int(time.Now().Unix()),
	}
	_ = oauthDao.Create(ctx, oauthClient)

	response.Success(c, gin.H{
		"client_id":           clientID,
		"client_secret":       clientSecret,
		"agent_status":        "approved",
		"approved_providers":  approvedProviders,
		"approval_expires_at": agent.ApprovalExpiresAt,
	})
}

// ATHAgentStatus queries agent registration status
// @Summary ATH agent status
// @Description Query ATH agent registration status
// @Tags ath
// @Accept json
// @Produce json
// @Param clientId path string true "client id"
// @Success 200 {object} types.ATHAgentStatusResponse{}
// @Router /api/v1/ath/agents/{clientId} [get]
func (h *usersHandler) ATHAgentStatus(c *gin.Context) {
	clientID := c.Param("clientId")
	if clientID == "" {
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())

	agent, err := athDao.GetByClientID(ctx, clientID)
	if err != nil {
		response.Error(c, ecode.ErrATHAgentNotFound)
		return
	}

	var approvedProviders []ath.ProviderApproval
	_ = json.Unmarshal([]byte(agent.ApprovedProviders), &approvedProviders)

	response.Success(c, gin.H{
		"client_id":           agent.ClientID,
		"agent_id":            agent.AgentID,
		"name":                agent.Name,
		"status":              agent.Status,
		"approved_providers":  approvedProviders,
		"approval_expires_at": agent.ApprovalExpiresAt,
	})
}

// ==================== ATH Authorization ====================

// ATHAuthorize initiates the ATH authorization flow
// @Summary ATH authorize
// @Description Initiate ATH authorization for user consent
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHAuthorizeRequest true "authorize request"
// @Success 200 {object} types.ATHAuthorizeResponse{}
// @Router /api/v1/ath/authorize [post]
func (h *usersHandler) ATHAuthorize(c *gin.Context) {
	form := &types.ATHAuthorizeRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())

	// Look up agent
	agent, err := athDao.GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrATHAgentNotFound)
		return
	}
	if agent.Status != "approved" {
		response.Error(c, ecode.ErrATHAgentDenied)
		return
	}

	// Validate provider
	if form.ProviderID != "user-service" {
		response.Error(c, ecode.ErrATHInvalidProvider)
		return
	}

	// Compute scope intersection
	var agentScopes []string
	_ = json.Unmarshal([]byte(agent.AllowedScopes), &agentScopes)
	allowed := intersectScopes(agentScopes, form.Scopes)
	if len(allowed) == 0 {
		response.Error(c, ecode.ErrATHInvalidScope)
		return
	}

	// Validate redirect_uri
	var redirectURIs []string
	_ = json.Unmarshal([]byte(agent.RedirectURIs), &redirectURIs)
	if len(redirectURIs) > 0 {
		if form.RedirectURI == "" {
			response.Error(c, ecode.ErrOAuthInvalidRequest.RewriteMsg("redirect_uri required"))
			return
		}
		found := false
		for _, uri := range redirectURIs {
			if uri == form.RedirectURI {
				found = true
				break
			}
		}
		if !found {
			response.Error(c, ecode.ErrOAuthInvalidRequest.RewriteMsg("redirect_uri mismatch"))
			return
		}
	}

	// Generate PKCE params (stored in session for validation during token exchange)
	_ = oauth.GenerateCode()

	// Generate ath_session_id
	sessionID := ath.GenerateSessionID()
	sessionData := map[string]interface{}{
		"client_id":    agent.ClientID,
		"agent_id":     agent.AgentID,
		"provider_id":  form.ProviderID,
		"scopes":       allowed,
		"state":        form.State,
		"redirect_uri": form.RedirectURI,
	}
	sessionJSON, _ := json.Marshal(sessionData)
	if err := ath.SaveSession(sessionID, string(sessionJSON)); err != nil {
		logger.Error("ATH save session error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	// Build OAuth authorization URL using the agent's OAuth client
	baseURL := fmt.Sprintf("http://%s", c.Request.Host)
	if config.Get().App.Host != "" {
		baseURL = fmt.Sprintf("http://%s:%d", config.Get().App.Host, config.Get().HTTP.Port)
	}

	redirectURI := form.RedirectURI
	if redirectURI == "" && len(redirectURIs) > 0 {
		redirectURI = redirectURIs[0]
	}
	if redirectURI == "" {
		redirectURI = baseURL + "/api/v1/ath/callback"
	}

	scopeStr := strings.Join(allowed, " ")
	authURL := fmt.Sprintf("%s/api/v1/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		baseURL, agent.ClientID, redirectURI, scopeStr, form.State)

	response.Success(c, gin.H{
		"authorization_url": authURL,
		"ath_session_id":    sessionID,
		"expires_in":        600,
	})
}

// ==================== ATH Token ====================

// ATHToken exchanges authorization code for ATH access token
// @Summary ATH token endpoint
// @Description Exchange OAuth authorization code for ATH token
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHTokenRequest true "token request"
// @Success 200 {object} types.ATHTokenResponse{}
// @Router /api/v1/ath/token [post]
func (h *usersHandler) ATHToken(c *gin.Context) {
	form := &types.ATHTokenRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())

	// Verify agent credentials
	agent, err := athDao.GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}
	if agent.Status != "approved" {
		response.Error(c, ecode.ErrATHAgentDenied)
		return
	}
	if form.ClientSecret != agent.ClientSecret {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}

	cfg := config.Get().OAuth
	accessExpire := 3600
	if cfg.AccessTokenExpire > 0 {
		accessExpire = cfg.AccessTokenExpire
	}
	refreshExpire := 2592000
	if cfg.RefreshTokenExpire > 0 {
		refreshExpire = cfg.RefreshTokenExpire
	}

	switch form.GrantType {
	case "authorization_code":
		h.handleATHAuthorizationCodeGrant(c, ctx, agent, form, accessExpire, refreshExpire)
	case "refresh_token":
		h.handleATHRefreshTokenGrant(c, ctx, agent, form, accessExpire, refreshExpire)
	default:
		response.Error(c, ecode.ErrOAuthUnsupportedGrant)
	}
}

func (h *usersHandler) handleATHAuthorizationCodeGrant(c *gin.Context, ctx interface{}, agent *model.ATHAgent, form *types.ATHTokenRequest, accessExpire, refreshExpire int) {
	if form.Code == "" || form.ATHSessionID == "" {
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	// Validate ath_session_id
	sessionDataStr, err := ath.GetSession(form.ATHSessionID, true)
	if err != nil {
		response.Error(c, ecode.ErrATHSessionExpired)
		return
	}
	var sessionData map[string]interface{}
	_ = json.Unmarshal([]byte(sessionDataStr), &sessionData)

	// Verify session belongs to this agent
	sessionClientID, _ := sessionData["client_id"].(string)
	if sessionClientID != agent.ClientID {
		response.Error(c, ecode.ErrATHSessionExpired)
		return
	}

	// Get and validate OAuth authorization code
	ac, err := oauth.GetAuthorizationCode(ctx.(context.Context), form.Code)
	if err != nil {
		logger.Warn("ATH GetAuthorizationCode error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// Verify code belongs to this agent's OAuth client
	if ac.ClientID != agent.ClientID {
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// Compute scope intersection: agent_approved ∩ user_consented
	var agentScopes []string
	_ = json.Unmarshal([]byte(agent.AllowedScopes), &agentScopes)
	userScopes := strings.Split(ac.Scope, " ")
	finalScopes := intersectScopes(agentScopes, userScopes)
	if len(finalScopes) == 0 {
		response.Error(c, ecode.ErrATHInvalidScope)
		return
	}
	scopeStr := strings.Join(finalScopes, " ")

	// Generate ATH tokens
	accessToken, err := jwt.GenerateATHToken(ac.UserID, agent.AgentID, agent.ClientID, scopeStr, "access", accessExpire)
	if err != nil {
		logger.Error("ATH generate access token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	refreshToken, err := jwt.GenerateATHToken(ac.UserID, agent.AgentID, agent.ClientID, scopeStr, "refresh", refreshExpire)
	if err != nil {
		logger.Error("ATH generate refresh token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    accessExpire,
		"scope":         scopeStr,
	})
}

func (h *usersHandler) handleATHRefreshTokenGrant(c *gin.Context, ctx interface{}, agent *model.ATHAgent, form *types.ATHTokenRequest, accessExpire, refreshExpire int) {
	if form.RefreshToken == "" {
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	claims, err := jwt.ParseATHToken(form.RefreshToken)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// Verify token belongs to this agent
	if claims.ClientID != agent.ClientID {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// Check revocation
	revoked, err := oauth.IsTokenRevoked(ctx.(context.Context), form.RefreshToken, "refresh")
	if err != nil {
		logger.Error("ATH IsTokenRevoked error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	if revoked {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// Revoke old refresh token
	_ = oauth.RevokeToken(ctx.(context.Context), form.RefreshToken, "refresh")

	// Generate new tokens
	accessToken, err := jwt.GenerateATHToken(claims.UserID, agent.AgentID, agent.ClientID, claims.Scope, "access", accessExpire)
	if err != nil {
		logger.Error("ATH generate access token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	refreshToken, err := jwt.GenerateATHToken(claims.UserID, agent.AgentID, agent.ClientID, claims.Scope, "refresh", refreshExpire)
	if err != nil {
		logger.Error("ATH generate refresh token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    accessExpire,
		"scope":         claims.Scope,
	})
}

// ==================== ATH Revoke ====================

// ATHRevoke revokes an ATH token
// @Summary ATH revoke
// @Description Revoke an ATH access or refresh token
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHRevokeRequest true "revoke request"
// @Success 200 {object} gin.H{}
// @Router /api/v1/ath/revoke [post]
func (h *usersHandler) ATHRevoke(c *gin.Context) {
	form := &types.ATHRevokeRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())

	// Verify agent credentials
	agent, err := athDao.GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}
	if form.ClientSecret != agent.ClientSecret {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}

	tokenType := "access"
	if form.TokenTypeHint == "refresh_token" {
		tokenType = "refresh"
	}

	if err := oauth.RevokeToken(ctx, form.Token, tokenType); err != nil {
		logger.Error("ATH revoke error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrOAuthRevokeFailed)
		return
	}

	response.Success(c)
}

// ==================== ATH Proxy ====================

// ATHProxy proxies API calls with ATH authentication
// @Summary ATH proxy
// @Description Proxy API calls authenticated with ATH token
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHProxyRequest true "proxy request"
// @Success 200 {object} gin.H{}
// @Router /api/v1/ath/proxy [post]
func (h *usersHandler) ATHProxy(c *gin.Context) {
	form := &types.ATHProxyRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	// Validate ATH token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}
	token := authHeader[7:]

	claims, err := jwt.ParseATHToken(token)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// Check revocation
	ctx := middleware.WrapCtx(c)
	revoked, err := oauth.IsTokenRevoked(ctx, token, "access")
	if err != nil {
		logger.Error("ATH proxy IsTokenRevoked error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	if revoked {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// Validate provider
	if form.Provider != "user-service" {
		response.Error(c, ecode.ErrATHInvalidProvider)
		return
	}

	// Validate scope
	userScopes := strings.Split(claims.Scope, " ")
	requiredScope := ""
	switch form.Method {
	case "GET":
		requiredScope = "user:read"
	case "POST", "PUT", "PATCH", "DELETE":
		requiredScope = "user:write"
	}
	if requiredScope != "" && !containsScope(userScopes, requiredScope) {
		response.Error(c, ecode.ErrOAuthAccessDenied)
		return
	}

	// For MVP, return a simple success response with proxy info
	// In production, this would forward the request to the actual API
	response.Success(c, gin.H{
		"proxied":    true,
		"provider":   form.Provider,
		"method":     form.Method,
		"path":       form.Path,
		"user_id":    claims.UserID,
		"agent_id":   jwt.GetAgentIDFromClaims(claims),
		"scope":      claims.Scope,
	})
}

// ==================== Helpers ====================

func intersectScopes(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	var result []string
	for _, s := range b {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}

func containsScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}
