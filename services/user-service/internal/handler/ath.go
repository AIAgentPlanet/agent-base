package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	c.JSON(http.StatusOK, ath.GetDiscoveryDocument(athBaseURL(c)))
}

// ATHServerIdentity publishes the gateway did:web verification key.
func (h *usersHandler) ATHServerIdentity(c *gin.Context) {
	if h.handshakeService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", "ATH handshake service is not configured")
		return
	}
	document, err := h.handshakeService.ServerIdentityDocument()
	if err != nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", err.Error())
		return
	}
	c.JSON(http.StatusOK, document)
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
	attestation := form.EffectiveAttestation()
	if attestation == "" || len(form.EffectiveProviders()) == 0 {
		athProtocolError(c, http.StatusBadRequest, "INVALID_ATTESTATION", "agent_attestation and requested_providers are required")
		return
	}

	verified, err := h.attestationVerifier.VerifyRegistration(
		c.Request.Context(), attestation, form.AgentID, athEndpointAudience(c, "/api/v1/ath/agents/register"),
	)
	if err != nil {
		logger.Warn("ATH attestation parse error", logger.Err(err), middleware.GCtxRequestIDField(c))
		athProtocolError(c, http.StatusUnauthorized, "INVALID_ATTESTATION", err.Error())
		return
	}
	agentID := verified.Claims.Subject

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())

	// Check if agent already registered
	existing, _ := athDao.GetByAgentID(ctx, agentID)
	if existing != nil {
		response.Error(c, ecode.ErrOAuthClientExists)
		return
	}

	if err := ath.CheckJTIReplay(verified.Claims.ID); err != nil {
		athProtocolError(c, http.StatusUnauthorized, "INVALID_ATTESTATION", "attestation jti replay detected")
		return
	}

	var scopes []string
	var approvedProviders []ath.ProviderApproval
	for _, requested := range form.EffectiveProviders() {
		approval := ath.ProviderApproval{ProviderID: requested.ProviderID}
		for _, scope := range requested.Scopes {
			if ath.SupportedScope(requested.ProviderID, scope) {
				approval.ApprovedScopes = append(approval.ApprovedScopes, scope)
				scopes = appendUnique(scopes, scope)
			} else {
				approval.DeniedScopes = append(approval.DeniedScopes, scope)
			}
		}
		if len(approval.ApprovedScopes) == 0 {
			approval.DenialReason = "provider or scopes are not supported"
		}
		approvedProviders = append(approvedProviders, approval)
	}
	if len(scopes) == 0 {
		athProtocolError(c, http.StatusForbidden, "PROVIDER_NOT_APPROVED", "no requested provider scopes were approved")
		return
	}
	scopesJSON, _ := json.Marshal(scopes)

	redirectURIs := []string{}
	if len(form.RedirectURIs) > 0 {
		redirectURIs = form.RedirectURIs
	}
	redirectURIsJSON, _ := json.Marshal(redirectURIs)

	developerJSON, _ := json.Marshal(form.Developer)

	approvedProvidersJSON, _ := json.Marshal(approvedProviders)

	clientID := ath.GenerateClientID()
	clientSecret := ath.GenerateClientSecret()

	agent := &model.ATHAgent{
		ClientID:          clientID,
		ClientSecret:      clientSecret,
		AgentID:           agentID,
		PublicKey:         verified.PublicKeyPEM,
		Name:              firstNonEmpty(form.Name, verified.Document.Name, agentID),
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
	if err := oauthDao.Create(ctx, oauthClient); err != nil {
		_ = athDao.DeleteByID(ctx, agent.ID)
		logger.Error("ATH OAuth client create error", logger.Err(err), middleware.GCtxRequestIDField(c))
		athProtocolError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create agent credentials")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"client_id":          clientID,
		"client_secret":      clientSecret,
		"agent_status":       "approved",
		"approved_providers": approvedProviders,
		"approval_expires":   time.Unix(int64(agent.ApprovalExpiresAt), 0).UTC().Format(time.RFC3339),
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

	c.JSON(http.StatusOK, gin.H{
		"client_id":          agent.ClientID,
		"agent_id":           agent.AgentID,
		"name":               agent.Name,
		"status":             agent.Status,
		"approved_providers": approvedProviders,
		"approval_expires":   time.Unix(int64(agent.ApprovalExpiresAt), 0).UTC().Format(time.RFC3339),
	})
}

// ==================== ATH Mutual Identity Handshake ====================

// ATHStartHandshake returns a signed server challenge.
// @Summary Start ATH handshake
// @Description Negotiate ATH capabilities and return a server-signed challenge
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHHandshakeStartRequest true "handshake announcement"
// @Success 200 {object} types.ATHHandshakeChallengeResponse{}
// @Router /api/v1/ath/handshakes [post]
func (h *usersHandler) ATHStartHandshake(c *gin.Context) {
	if h.handshakeService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", "ATH handshake service is not configured")
		return
	}
	form := &types.ATHHandshakeStartRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)
	agent, err := dao.NewATHAgentDao(database.GetDB()).GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrATHAgentNotFound)
		return
	}
	if agent.Status != "approved" || (agent.ApprovalExpiresAt > 0 && time.Now().Unix() > int64(agent.ApprovalExpiresAt)) {
		response.Error(c, ecode.ErrATHAgentDenied)
		return
	}
	if agent.AgentID != form.ClientDID {
		athProtocolError(c, http.StatusUnauthorized, "IDENTITY_MISMATCH", "client_did does not match the registered agent")
		return
	}

	handshake, err := h.handshakeService.Start(c.Request.Context(), ath.StartHandshakeInput{
		ClientID: form.ClientID, ClientDID: form.ClientDID,
		Versions: form.Versions, Capabilities: form.Capabilities,
		Nonce: form.Nonce, EphemeralKey: form.EphemeralKey, Timestamp: form.Timestamp,
	})
	if err != nil {
		athProtocolError(c, http.StatusBadRequest, "HANDSHAKE_REJECTED", err.Error())
		return
	}
	h.recordATHAudit(c, ath.AuditEvent{
		EventType: ath.AuditEventHandshakeStarted,
		ClientID:  agent.ClientID, AgentID: agent.AgentID, HandshakeID: handshake.ID,
		Payload: gin.H{
			"version": handshake.Version, "capabilities": handshake.Capabilities,
			"expires_at": handshake.ExpiresAt,
		},
	})
	c.JSON(http.StatusOK, &types.ATHHandshakeChallengeResponse{
		HandshakeID: handshake.ID, Status: handshake.State,
		ServerDID: handshake.ServerDID, ServerKeyID: handshake.ServerKeyID,
		ServerPublicKey: handshake.ServerPublicKeyPEM,
		Version:         handshake.Version, Capabilities: handshake.Capabilities,
		ClientNonce: handshake.ClientNonce, ServerNonce: handshake.ServerNonce,
		ClientEphemeralKey: handshake.ClientEphemeralKey,
		ServerEphemeralKey: handshake.ServerEphemeralKey,
		Signature:          handshake.ServerSignature, Timestamp: handshake.CreatedAt,
		ExpiresAt: handshake.ExpiresAt,
	})
}

// ATHCompleteHandshake verifies the client proof signature.
// @Summary Complete ATH handshake
// @Description Verify the client signature over the server challenge
// @Tags ath
// @Accept json
// @Produce json
// @Param handshakeId path string true "handshake id"
// @Param data body types.ATHHandshakeProofRequest true "client identity proof"
// @Success 200 {object} types.ATHHandshakeStatusResponse{}
// @Router /api/v1/ath/handshakes/{handshakeId}/proof [post]
func (h *usersHandler) ATHCompleteHandshake(c *gin.Context) {
	if h.handshakeService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", "ATH handshake service is not configured")
		return
	}
	form := &types.ATHHandshakeProofRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}
	ctx := middleware.WrapCtx(c)
	agent, err := dao.NewATHAgentDao(database.GetDB()).GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrATHAgentNotFound)
		return
	}
	handshake, err := h.handshakeService.Complete(c.Request.Context(), ath.CompleteHandshakeInput{
		HandshakeID: c.Param("handshakeId"), ClientID: form.ClientID,
		ClientDID: agent.AgentID, PublicKeyPEM: agent.PublicKey,
		Signature: form.Signature, Timestamp: form.Timestamp,
	})
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ath.ErrHandshakeNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, ath.ErrHandshakeStateConflict) {
			status = http.StatusConflict
		}
		athProtocolError(c, status, "INVALID_CLIENT_PROOF", err.Error())
		return
	}
	h.recordATHAudit(c, ath.AuditEvent{
		EventType: ath.AuditEventHandshakeVerified,
		ClientID:  agent.ClientID, AgentID: agent.AgentID, HandshakeID: handshake.ID,
		Payload: gin.H{
			"verified_at": handshake.VerifiedAt, "expires_at": handshake.ExpiresAt,
			"session_key_algorithm": "ECDH-P256+HKDF-SHA256",
		},
	})
	c.JSON(http.StatusOK, handshakeStatusResponse(handshake))
}

// ATHHandshakeStatus returns the state of a handshake owned by the client.
// @Summary ATH handshake status
// @Tags ath
// @Produce json
// @Param handshakeId path string true "handshake id"
// @Param client_id query string true "client id"
// @Success 200 {object} types.ATHHandshakeStatusResponse{}
// @Router /api/v1/ath/handshakes/{handshakeId} [get]
func (h *usersHandler) ATHHandshakeStatus(c *gin.Context) {
	if h.handshakeService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", "ATH handshake service is not configured")
		return
	}
	clientID := c.Query("client_id")
	if clientID == "" {
		response.Error(c, ecode.InvalidParams)
		return
	}
	handshake, err := h.handshakeService.Get(c.Request.Context(), c.Param("handshakeId"))
	if err != nil || handshake.ClientID != clientID {
		athProtocolError(c, http.StatusNotFound, "HANDSHAKE_NOT_FOUND", "ATH handshake not found or expired")
		return
	}
	c.JSON(http.StatusOK, handshakeStatusResponse(handshake))
}

func (h *usersHandler) ATHAuditHead(c *gin.Context) {
	if h.auditService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "ATH audit service is not configured")
		return
	}
	head, err := h.auditService.Head(c.Request.Context())
	if err != nil {
		athProtocolError(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", err.Error())
		return
	}
	c.JSON(http.StatusOK, head)
}

func (h *usersHandler) ATHAuditQuery(c *gin.Context) {
	form := &types.ATHAuditQueryRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}
	agent, ok := h.authenticateATHAuditClient(c, form.ClientID, form.ClientSecret)
	if !ok {
		return
	}
	if h.auditService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "ATH audit service is not configured")
		return
	}
	records, err := h.auditService.ListByHandshake(c.Request.Context(), form.HandshakeID, form.Limit)
	if err != nil {
		athProtocolError(c, http.StatusInternalServerError, "AUDIT_ERROR", err.Error())
		return
	}
	for _, record := range records {
		if record.ClientID != agent.ClientID {
			athProtocolError(c, http.StatusNotFound, "AUDIT_NOT_FOUND", "audit records not found")
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"records": records})
}

func (h *usersHandler) ATHAuditVerify(c *gin.Context) {
	form := &types.ATHAuditVerifyRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}
	if _, ok := h.authenticateATHAuditClient(c, form.ClientID, form.ClientSecret); !ok {
		return
	}
	if h.auditService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "ATH audit service is not configured")
		return
	}
	result, err := h.auditService.Verify(c.Request.Context())
	if err != nil {
		athProtocolError(c, http.StatusInternalServerError, "AUDIT_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *usersHandler) ATHAnchorStatus(c *gin.Context) {
	form := &types.ATHAnchorAdminRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}
	if _, ok := h.authenticateATHAuditClient(c, form.ClientID, form.ClientSecret); !ok {
		return
	}
	worker := ath.DefaultAnchorWorker()
	if worker == nil {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}
	status, err := worker.Status(c.Request.Context())
	if err != nil {
		athProtocolError(c, http.StatusInternalServerError, "ANCHOR_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"configured": worker.Configured(), "outbox": status})
}

func (h *usersHandler) ATHAnchorRetry(c *gin.Context) {
	form := &types.ATHAnchorAdminRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}
	if _, ok := h.authenticateATHAuditClient(c, form.ClientID, form.ClientSecret); !ok {
		return
	}
	worker := ath.DefaultAnchorWorker()
	if worker == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "ANCHOR_UNAVAILABLE", "ATH anchor worker is not configured")
		return
	}
	count, err := worker.RetryFailed(c.Request.Context())
	if err != nil {
		athProtocolError(c, http.StatusInternalServerError, "ANCHOR_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"requeued": count})
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
	if len(form.State) < 22 {
		athProtocolError(c, http.StatusBadRequest, "OAUTH_ERROR", "state must contain at least 128 bits of entropy")
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
	if h.handshakeService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", "ATH handshake service is not configured")
		return
	}
	if _, err := h.handshakeService.RequireVerified(ctx, form.HandshakeID, agent.AgentID, agent.ClientID); err != nil {
		athProtocolError(c, http.StatusUnauthorized, "HANDSHAKE_REQUIRED", err.Error())
		return
	}
	attestation := requestAttestation(c, form.AgentAttestation)
	claims, err := h.attestationVerifier.VerifyRegistered(
		attestation, agent, athEndpointAudience(c, "/api/v1/ath/authorize"),
	)
	if err != nil {
		athProtocolError(c, http.StatusUnauthorized, "INVALID_ATTESTATION", err.Error())
		return
	}
	if err := ath.CheckJTIReplay(claims.ID); err != nil {
		athProtocolError(c, http.StatusUnauthorized, "INVALID_ATTESTATION", "attestation jti replay detected")
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
	redirectURI := form.EffectiveRedirectURI()
	var redirectURIs []string
	_ = json.Unmarshal([]byte(agent.RedirectURIs), &redirectURIs)
	if len(redirectURIs) > 0 {
		if redirectURI == "" {
			response.Error(c, ecode.ErrOAuthInvalidRequest.RewriteMsg("redirect_uri required"))
			return
		}
		found := false
		for _, uri := range redirectURIs {
			if uri == redirectURI {
				found = true
				break
			}
		}
		if !found {
			response.Error(c, ecode.ErrOAuthInvalidRequest.RewriteMsg("redirect_uri mismatch"))
			return
		}
	} else if redirectURI != "" {
		athProtocolError(c, http.StatusBadRequest, "OAUTH_ERROR", "user_redirect_uri is not allowed for this agent")
		return
	}

	pkceVerifier := ath.GenerateClientSecret()
	pkceDigest := sha256.Sum256([]byte(pkceVerifier))
	pkceChallenge := base64.RawURLEncoding.EncodeToString(pkceDigest[:])

	// Generate ath_session_id
	sessionID := ath.GenerateSessionID()
	sessionData := map[string]interface{}{
		"client_id":     agent.ClientID,
		"agent_id":      agent.AgentID,
		"handshake_id":  form.HandshakeID,
		"provider_id":   form.ProviderID,
		"scopes":        allowed,
		"state":         form.State,
		"redirect_uri":  redirectURI,
		"resource":      form.Resource,
		"pkce_verifier": pkceVerifier,
	}
	sessionJSON, _ := json.Marshal(sessionData)
	if err := ath.SaveSession(sessionID, string(sessionJSON)); err != nil {
		logger.Error("ATH save session error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	h.recordATHAudit(c, ath.AuditEvent{
		EventType: ath.AuditEventAuthorizationStarted,
		ClientID:  agent.ClientID, AgentID: agent.AgentID, HandshakeID: form.HandshakeID,
		Payload: gin.H{
			"provider_id": form.ProviderID, "scopes": allowed,
			"resource": form.Resource,
		},
	})

	// Build OAuth authorization URL using the agent's OAuth client
	baseURL := athBaseURL(c)

	if redirectURI == "" && len(redirectURIs) > 0 {
		redirectURI = redirectURIs[0]
	}
	if redirectURI == "" {
		redirectURI = baseURL + "/api/v1/ath/callback"
	}

	scopeStr := strings.Join(allowed, " ")
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", agent.ClientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("scope", scopeStr)
	values.Set("state", form.State)
	values.Set("code_challenge", pkceChallenge)
	values.Set("code_challenge_method", "S256")
	authURL := baseURL + "/api/v1/oauth/authorize?" + values.Encode()

	c.JSON(http.StatusOK, gin.H{
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
	attestation := requestAttestation(c, form.AgentAttestation)
	claims, err := h.attestationVerifier.VerifyRegistered(
		attestation, agent, athEndpointAudience(c, "/api/v1/ath/token"),
	)
	if err != nil {
		athProtocolError(c, http.StatusUnauthorized, "INVALID_ATTESTATION", err.Error())
		return
	}
	if err := ath.CheckJTIReplay(claims.ID); err != nil {
		athProtocolError(c, http.StatusUnauthorized, "INVALID_ATTESTATION", "attestation jti replay detected")
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
	sessionDataStr, err := ath.GetSession(form.ATHSessionID, false)
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
	sessionState, _ := sessionData["state"].(string)
	if sessionState == "" || ac.State != sessionState {
		athProtocolError(c, http.StatusBadRequest, "STATE_MISMATCH", "OAuth state does not match ATH session")
		return
	}
	sessionRedirectURI, _ := sessionData["redirect_uri"].(string)
	if ac.RedirectURI != sessionRedirectURI {
		athProtocolError(c, http.StatusBadRequest, "OAUTH_ERROR", "redirect URI does not match ATH session")
		return
	}
	pkceVerifier, _ := sessionData["pkce_verifier"].(string)
	if !oauth.ValidatePKCE(pkceVerifier, ac.PKCECodeChallenge, ac.PKCECodeChallengeMethod) {
		athProtocolError(c, http.StatusBadRequest, "OAUTH_ERROR", "PKCE validation failed")
		return
	}

	// Compute scope intersection: agent_approved ∩ user_consented ∩ requested
	var agentScopes []string
	_ = json.Unmarshal([]byte(agent.AllowedScopes), &agentScopes)
	userScopes := strings.Split(ac.Scope, " ")
	requestedScopes := interfaceStrings(sessionData["scopes"])
	finalScopes := intersectScopes(intersectScopes(agentScopes, userScopes), requestedScopes)
	if len(finalScopes) == 0 {
		response.Error(c, ecode.ErrATHInvalidScope)
		return
	}
	scopeStr := strings.Join(finalScopes, " ")
	providerID, _ := sessionData["provider_id"].(string)
	handshakeID, _ := sessionData["handshake_id"].(string)
	verifiedHandshake, err := h.handshakeService.RequireVerified(ctx.(context.Context), handshakeID, agent.AgentID, agent.ClientID)
	if err != nil {
		athProtocolError(c, http.StatusUnauthorized, "HANDSHAKE_REQUIRED", err.Error())
		return
	}
	accessExpire = limitTokenLifetime(accessExpire, verifiedHandshake.ExpiresAt)
	if accessExpire <= 0 {
		athProtocolError(c, http.StatusUnauthorized, "HANDSHAKE_REQUIRED", "ATH secure session has expired")
		return
	}

	// Generate ATH tokens
	accessToken, err := jwt.GenerateATHToken(ac.UserID, agent.AgentID, agent.ClientID, providerID, form.ATHSessionID, handshakeID, scopeStr, "access", accessExpire)
	if err != nil {
		logger.Error("ATH generate access token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	accessClaims, _ := jwt.ParseATHToken(accessToken)
	tokenID := ""
	if accessClaims != nil {
		tokenID = accessClaims.ID
	}
	h.recordATHAudit(c, ath.AuditEvent{
		EventType: ath.AuditEventTokenIssued,
		ClientID:  agent.ClientID, AgentID: agent.AgentID,
		HandshakeID: handshakeID, TokenID: tokenID,
		Payload: gin.H{
			"provider_id": providerID, "scopes": finalScopes,
			"user_id": ac.UserID, "expires_in": accessExpire,
		},
	})

	if err := ath.DeleteSession(form.ATHSessionID); err != nil {
		logger.Warn("ATH delete session error", logger.Err(err), middleware.GCtxRequestIDField(c))
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":     accessToken,
		"token_type":       "Bearer",
		"expires_in":       accessExpire,
		"effective_scopes": finalScopes,
		"provider_id":      providerID,
		"agent_id":         agent.AgentID,
		"handshake_id":     handshakeID,
		"scope_intersection": gin.H{
			"agent_approved": agentScopes,
			"user_consented": userScopes,
			"requested":      requestedScopes,
			"effective":      finalScopes,
		},
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
	if h.handshakeService == nil {
		athProtocolError(c, http.StatusServiceUnavailable, "HANDSHAKE_UNAVAILABLE", "ATH handshake service is not configured")
		return
	}
	verifiedHandshake, err := h.handshakeService.RequireVerified(
		ctx.(context.Context), claims.HandshakeID, agent.AgentID, agent.ClientID,
	)
	if err != nil {
		athProtocolError(c, http.StatusUnauthorized, "HANDSHAKE_REQUIRED", err.Error())
		return
	}
	accessExpire = limitTokenLifetime(accessExpire, verifiedHandshake.ExpiresAt)
	if accessExpire <= 0 {
		athProtocolError(c, http.StatusUnauthorized, "HANDSHAKE_REQUIRED", "ATH secure session has expired")
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
	accessToken, err := jwt.GenerateATHToken(claims.UserID, agent.AgentID, agent.ClientID, claims.ProviderID, claims.SessionID, claims.HandshakeID, claims.Scope, "access", accessExpire)
	if err != nil {
		logger.Error("ATH generate access token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	refreshToken, err := jwt.GenerateATHToken(claims.UserID, agent.AgentID, agent.ClientID, claims.ProviderID, claims.SessionID, claims.HandshakeID, claims.Scope, "refresh", refreshExpire)
	if err != nil {
		logger.Error("ATH generate refresh token error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	newAccessClaims, _ := jwt.ParseATHToken(accessToken)
	newTokenID := ""
	if newAccessClaims != nil {
		newTokenID = newAccessClaims.ID
	}
	h.recordATHAudit(c, ath.AuditEvent{
		EventType: ath.AuditEventTokenRefreshed,
		ClientID:  agent.ClientID, AgentID: agent.AgentID,
		HandshakeID: claims.HandshakeID, TokenID: newTokenID,
		Payload: gin.H{
			"provider_id": claims.ProviderID, "scope": claims.Scope,
			"user_id": claims.UserID, "expires_in": accessExpire,
		},
	})

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
	revokedClaims, _ := jwt.ParseATHToken(form.Token)
	if revokedClaims != nil {
		h.recordATHAudit(c, ath.AuditEvent{
			EventType: ath.AuditEventTokenRevoked,
			ClientID:  agent.ClientID, AgentID: agent.AgentID,
			HandshakeID: revokedClaims.HandshakeID, TokenID: revokedClaims.ID,
			Payload: gin.H{"token_type": tokenType},
		})
	}

	response.Success(c)
}

// ATHIntrospect returns active status and binding claims for an ATH token.
// @Summary ATH token introspection
// @Description Introspect an ATH access or refresh token and check revocation.
// @Tags ath
// @Accept json
// @Produce json
// @Param data body types.ATHIntrospectRequest true "introspection request"
// @Success 200 {object} types.ATHIntrospectResponse{}
// @Router /api/v1/ath/introspect [post]
func (h *usersHandler) ATHIntrospect(c *gin.Context) {
	form := &types.ATHIntrospectRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)
	athDao := dao.NewATHAgentDao(database.GetDB())
	agent, err := athDao.GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}
	if form.ClientSecret != agent.ClientSecret {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}

	claims, err := jwt.ParseATHToken(form.Token)
	if err != nil {
		c.JSON(http.StatusOK, types.ATHIntrospectResponse{Active: false})
		return
	}
	if claims.ClientID != agent.ClientID {
		c.JSON(http.StatusOK, types.ATHIntrospectResponse{Active: false})
		return
	}
	tokenType := "access"
	if form.TokenTypeHint == "refresh_token" || claims.Type == "ath_refresh" {
		tokenType = "refresh"
	}
	revoked, err := oauth.IsTokenRevoked(ctx.(context.Context), form.Token, tokenType)
	if err != nil {
		logger.Error("ATH introspect IsTokenRevoked error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	if revoked {
		c.JSON(http.StatusOK, types.ATHIntrospectResponse{Active: false})
		return
	}

	out := types.ATHIntrospectResponse{
		Active: true, TokenType: claims.Type, ClientID: claims.ClientID, AgentID: claims.AgentID,
		UserID: claims.UserID, ProviderID: claims.ProviderID, SessionID: claims.SessionID,
		HandshakeID: claims.HandshakeID, Scope: claims.Scope, Scopes: strings.Fields(claims.Scope),
		JTI: claims.ID,
	}
	if claims.ExpiresAt != nil {
		out.ExpiresAt = claims.ExpiresAt.Unix()
	}
	if claims.IssuedAt != nil {
		out.IssuedAt = claims.IssuedAt.Unix()
	}
	if claims.NotBefore != nil {
		out.NotBefore = claims.NotBefore.Unix()
	}
	c.JSON(http.StatusOK, out)
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
	if claims.HandshakeID == "" || h.handshakeService == nil {
		athProtocolError(c, http.StatusUnauthorized, "SESSION_KEY_REQUIRED", "ATH token is not bound to a session key")
		return
	}
	if err := h.handshakeService.VerifyRequestIntegrity(ctx, ath.IntegrityInput{
		HandshakeID: claims.HandshakeID, TokenID: claims.ID,
		Provider: form.Provider, Method: form.Method, Path: form.Path, Body: form.Body,
		Timestamp: form.RequestTimestamp, Nonce: form.RequestNonce,
		Signature: form.RequestSignature,
	}); err != nil {
		status := http.StatusUnauthorized
		code := "INVALID_REQUEST_SIGNATURE"
		if errors.Is(err, ath.ErrRequestReplay) {
			status = http.StatusConflict
			code = "REQUEST_REPLAY"
		}
		athProtocolError(c, status, code, err.Error())
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
	bodyDigest := sha256.Sum256([]byte(form.Body))
	h.recordATHAudit(c, ath.AuditEvent{
		EventType: ath.AuditEventProxyAllowed,
		ClientID:  claims.ClientID, AgentID: claims.AgentID,
		HandshakeID: claims.HandshakeID, TokenID: claims.ID,
		Payload: gin.H{
			"provider": form.Provider, "method": form.Method, "path": form.Path,
			"body_sha256": fmt.Sprintf("%x", bodyDigest[:]),
		},
	})

	// For MVP, return a simple success response with proxy info
	// In production, this would forward the request to the actual API
	response.Success(c, gin.H{
		"proxied":  true,
		"provider": form.Provider,
		"method":   form.Method,
		"path":     form.Path,
		"user_id":  claims.UserID,
		"agent_id": jwt.GetAgentIDFromClaims(claims),
		"scope":    claims.Scope,
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

func athBaseURL(c *gin.Context) string {
	if configured := strings.TrimRight(config.Get().ATH.BaseURL, "/"); configured != "" {
		return configured
	}
	scheme := "http"
	if c.Request.TLS != nil || config.Get().HTTP.TLS.EnableMode != "" {
		scheme = "https"
	}
	host := c.Request.Host
	if config.Get().App.Host != "" {
		host = fmt.Sprintf("%s:%d", config.Get().App.Host, config.Get().HTTP.Port)
	}
	return scheme + "://" + host
}

func athEndpointAudience(c *gin.Context, path string) string {
	return athBaseURL(c) + path
}

func requestAttestation(c *gin.Context, bodyValue string) string {
	if bodyValue != "" {
		return bodyValue
	}
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}

func athProtocolError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"code": code, "message": message})
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func interfaceStrings(value interface{}) []string {
	raw, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func handshakeStatusResponse(handshake *ath.Handshake) *types.ATHHandshakeStatusResponse {
	return &types.ATHHandshakeStatusResponse{
		HandshakeID: handshake.ID, ClientDID: handshake.ClientDID,
		ServerDID: handshake.ServerDID, Status: handshake.State,
		Version: handshake.Version, CreatedAt: handshake.CreatedAt,
		ExpiresAt: handshake.ExpiresAt, VerifiedAt: handshake.VerifiedAt,
		SessionKeyEstablished: handshake.SessionKey != "",
	}
}

func limitTokenLifetime(configuredSeconds int, sessionExpiresAt int64) int {
	remaining := int(time.Until(time.Unix(sessionExpiresAt, 0)).Seconds())
	if remaining < configuredSeconds {
		return remaining
	}
	return configuredSeconds
}

func (h *usersHandler) authenticateATHAuditClient(c *gin.Context, clientID, clientSecret string) (*model.ATHAgent, bool) {
	agent, err := dao.NewATHAgentDao(database.GetDB()).GetByClientID(middleware.WrapCtx(c), clientID)
	if err != nil || agent.ClientSecret != clientSecret || agent.Status != "approved" {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return nil, false
	}
	return agent, true
}

func (h *usersHandler) recordATHAudit(c *gin.Context, event ath.AuditEvent) {
	if h.auditService == nil {
		c.Header("X-ATH-Audit-Status", "failed")
		logger.Error("ATH audit service unavailable", middleware.GCtxRequestIDField(c))
		return
	}
	record, err := h.auditService.Append(c.Request.Context(), event)
	if err != nil {
		c.Header("X-ATH-Audit-Status", "failed")
		logger.Error(
			"ATH audit append error",
			logger.Err(err),
			logger.String("event_type", event.EventType),
			middleware.GCtxRequestIDField(c),
		)
		return
	}
	c.Header("X-ATH-Audit-Status", "recorded")
	c.Header("X-ATH-Audit-Event-ID", record.EventID)
	c.Header("X-ATH-Audit-Record-Hash", record.RecordHash)
}
