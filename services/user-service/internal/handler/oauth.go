package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/go-dev-frame/sponge/pkg/gin/middleware"
	"github.com/go-dev-frame/sponge/pkg/gin/response"
	"github.com/go-dev-frame/sponge/pkg/logger"
	"github.com/go-dev-frame/sponge/pkg/sgorm/query"

	"agent-base/services/user-service/internal/dao"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/ecode"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/pkg/jwt"
	"agent-base/services/user-service/internal/pkg/oauth"
	"agent-base/services/user-service/internal/types"
)

// ==================== OAuth Client Management ====================

// CreateOAuthClient create a new oauth client
// @Summary Create OAuth client
// @Description Register a new OAuth client application
// @Tags oauth
// @Accept json
// @Produce json
// @Param data body types.CreateOAuthClientRequest true "client information"
// @Success 200 {object} types.CreateOAuthClientReply{}
// @Router /api/v1/oauth/clients [post]
// @Security BearerAuth
func (h *usersHandler) CreateOAuthClient(c *gin.Context) {
	form := &types.CreateOAuthClientRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	userID, err := getUserIDFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	ctx := middleware.WrapCtx(c)

	// check if client name exists for this user
	_, err = oauthDao.GetByCondition(ctx, &query.Conditions{
		Columns: []query.Column{
			{Name: "name", Value: form.Name},
			{Name: "user_id", Value: userID},
		},
	})
	if err == nil {
		response.Error(c, ecode.ErrOAuthClientExists)
		return
	}

	redirectURIs, _ := json.Marshal(form.RedirectURIs)
	allowedGrants := []string{"authorization_code"}
	if len(form.AllowedGrants) > 0 {
		allowedGrants = form.AllowedGrants
	}
	allowedScopes := []string{"profile"}
	if len(form.AllowedScopes) > 0 {
		allowedScopes = form.AllowedScopes
	}

	grants, _ := json.Marshal(allowedGrants)
	scopes, _ := json.Marshal(allowedScopes)

	client := &model.OAuthClient{
		ClientID:      oauth.GenerateClientID(),
		ClientSecret:  oauth.GenerateClientSecret(),
		Name:          form.Name,
		RedirectURIs:  string(redirectURIs),
		AllowedGrants: string(grants),
		AllowedScopes: string(scopes),
		UserID:        userID,
		Status:        1, // active
		CreatedAt:     int(time.Now().Unix()),
		UpdatedAt:     int(time.Now().Unix()),
	}

	if err := oauthDao.Create(ctx, client); err != nil {
		logger.Error("Create oauth client error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{
		"id":            client.ID,
		"client_id":     client.ClientID,
		"client_secret": client.ClientSecret,
	})
}

// ListOAuthClients list oauth clients of current user
// @Summary List OAuth clients
// @Description List all OAuth clients registered by current user
// @Tags oauth
// @Accept json
// @Produce json
// @Param data body types.ListOAuthClientsRequest true "query parameters"
// @Success 200 {object} types.ListOAuthClientsReply{}
// @Router /api/v1/oauth/clients [post]
// @Security BearerAuth
func (h *usersHandler) ListOAuthClients(c *gin.Context) {
	form := &types.ListOAuthClientsRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	userID, err := getUserIDFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	// ensure query only returns current user's clients
	form.Columns = append(form.Columns, query.Column{
		Name:  "user_id",
		Value: userID,
	})

	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	ctx := middleware.WrapCtx(c)

	clients, total, err := oauthDao.GetByColumns(ctx, &form.Params)
	if err != nil {
		logger.Error("GetByColumns error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	data := make([]types.OAuthClientObjDetail, 0, len(clients))
	for _, client := range clients {
		var uris, grants, scopes []string
		_ = json.Unmarshal([]byte(client.RedirectURIs), &uris)
		_ = json.Unmarshal([]byte(client.AllowedGrants), &grants)
		_ = json.Unmarshal([]byte(client.AllowedScopes), &scopes)
		data = append(data, types.OAuthClientObjDetail{
			ID:            client.ID,
			ClientID:      client.ClientID,
			Name:          client.Name,
			RedirectURIs:  uris,
			AllowedGrants: grants,
			AllowedScopes: scopes,
			Status:        client.Status,
			CreatedAt:     client.CreatedAt,
			UpdatedAt:     client.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"clients": data,
		"total":   total,
	})
}

// UpdateOAuthClient update an oauth client
// @Summary Update OAuth client
// @Description Update OAuth client information
// @Tags oauth
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Param data body types.UpdateOAuthClientRequest true "client information"
// @Success 200 {object} types.UpdateOAuthClientReply{}
// @Router /api/v1/oauth/clients/{id} [put]
// @Security BearerAuth
func (h *usersHandler) UpdateOAuthClient(c *gin.Context) {
	_, id, isAbort := getUsersIDFromPath(c)
	if isAbort {
		response.Error(c, ecode.InvalidParams)
		return
	}

	form := &types.UpdateOAuthClientRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	userID, err := getUserIDFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	ctx := middleware.WrapCtx(c)

	// verify ownership
	client, err := oauthDao.GetByID(ctx, id)
	if err != nil {
		response.Error(c, ecode.ErrOAuthClientNotFound)
		return
	}
	if client.UserID != userID {
		response.Error(c, ecode.ErrOAuthAccessDenied)
		return
	}

	update := &model.OAuthClient{
		ID:        id,
		UpdatedAt: int(time.Now().Unix()),
	}
	if form.Name != "" {
		update.Name = form.Name
	}
	if len(form.RedirectURIs) > 0 {
		uris, _ := json.Marshal(form.RedirectURIs)
		update.RedirectURIs = string(uris)
	}
	if len(form.AllowedGrants) > 0 {
		grants, _ := json.Marshal(form.AllowedGrants)
		update.AllowedGrants = string(grants)
	}
	if len(form.AllowedScopes) > 0 {
		scopes, _ := json.Marshal(form.AllowedScopes)
		update.AllowedScopes = string(scopes)
	}
	if form.Status != 0 {
		update.Status = form.Status
	}

	if err := oauthDao.UpdateByID(ctx, update); err != nil {
		logger.Error("UpdateByID error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// DeleteOAuthClient delete an oauth client
// @Summary Delete OAuth client
// @Description Delete an OAuth client by id
// @Tags oauth
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} types.DeleteOAuthClientByIDReply{}
// @Router /api/v1/oauth/clients/{id} [delete]
// @Security BearerAuth
func (h *usersHandler) DeleteOAuthClient(c *gin.Context) {
	_, id, isAbort := getUsersIDFromPath(c)
	if isAbort {
		response.Error(c, ecode.InvalidParams)
		return
	}

	userID, err := getUserIDFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	ctx := middleware.WrapCtx(c)

	// verify ownership
	client, err := oauthDao.GetByID(ctx, id)
	if err != nil {
		response.Error(c, ecode.ErrOAuthClientNotFound)
		return
	}
	if client.UserID != userID {
		response.Error(c, ecode.ErrOAuthAccessDenied)
		return
	}

	if err := oauthDao.DeleteByID(ctx, id); err != nil {
		logger.Error("DeleteByID error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// ==================== OAuth 2.0 Endpoints ====================

// OAuthAuthorize authorization endpoint
// @Summary OAuth authorization endpoint
// @Description Initiate OAuth 2.0 authorization code flow, user must be logged in
// @Tags oauth
// @Accept json
// @Produce json
// @Param response_type query string true "response type, must be 'code'"
// @Param client_id query string true "client id"
// @Param redirect_uri query string true "redirect uri"
// @Param scope query string false "requested scopes"
// @Param state query string false "state parameter"
// @Param code_challenge query string false "PKCE code challenge"
// @Param code_challenge_method query string false "PKCE code challenge method: plain or S256"
// @Success 302 {string} string "redirect to client redirect_uri with code"
// @Router /api/v1/oauth/authorize [get]
// @Security BearerAuth
func (h *usersHandler) OAuthAuthorize(c *gin.Context) {
	form := &types.OAuthAuthorizeRequest{}
	if err := c.ShouldBindQuery(form); err != nil {
		logger.Warn("ShouldBindQuery error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		oAuthRedirectError(c, form.RedirectURI, "invalid_request", "invalid request parameters", form.State)
		return
	}

	// user must be logged in
	userID, err := getUserIDFromContext(c)
	if err != nil {
		// redirect with error instead of JSON
		oAuthRedirectError(c, form.RedirectURI, "access_denied", "user not authenticated", form.State)
		return
	}

	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	ctx := middleware.WrapCtx(c)

	client, err := oauthDao.GetByClientID(ctx, form.ClientID)
	if err != nil {
		oAuthRedirectError(c, form.RedirectURI, "invalid_client", "client not found", form.State)
		return
	}
	if client.Status != 1 {
		oAuthRedirectError(c, form.RedirectURI, "invalid_client", "client is inactive", form.State)
		return
	}

	// validate redirect_uri
	if !oauth.ValidateRedirectURI(client, form.RedirectURI) {
		oAuthRedirectError(c, form.RedirectURI, "invalid_request", "invalid redirect_uri", form.State)
		return
	}

	// validate scope
	if !oauth.ValidateScope(client, form.Scope) {
		oAuthRedirectError(c, form.RedirectURI, "invalid_scope", "invalid scope", form.State)
		return
	}

	// generate authorization code
	code := oauth.GenerateCode()
	ac := &oauth.AuthorizationCode{
		Code:                    code,
		ClientID:                form.ClientID,
		UserID:                  userID,
		RedirectURI:             form.RedirectURI,
		Scope:                   form.Scope,
		State:                   form.State,
		PKCECodeChallenge:       form.CodeChallenge,
		PKCECodeChallengeMethod: form.CodeChallengeMethod,
		ExpiresAt:               time.Now().Add(oauth.CodeTTL).Unix(),
	}

	if err := oauth.SaveAuthorizationCode(ctx, ac); err != nil {
		logger.Error("SaveAuthorizationCode error", logger.Err(err), middleware.GCtxRequestIDField(c))
		oAuthRedirectError(c, form.RedirectURI, "server_error", "failed to generate code", form.State)
		return
	}

	// redirect with code
	redirectURL := fmt.Sprintf("%s?code=%s", form.RedirectURI, code)
	if form.State != "" {
		redirectURL += "&state=" + form.State
	}
	c.Redirect(http.StatusFound, redirectURL)
}

// OAuthToken token endpoint
// @Summary OAuth token endpoint
// @Description Exchange authorization code for tokens or refresh tokens
// @Tags oauth
// @Accept json
// @Produce json
// @Param data body types.OAuthTokenRequest true "token request"
// @Success 200 {object} types.OAuthTokenReply{}
// @Router /api/v1/oauth/token [post]
func (h *usersHandler) OAuthToken(c *gin.Context) {
	form := &types.OAuthTokenRequest{}
	if err := c.ShouldBind(form); err != nil {
		logger.Warn("ShouldBind error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	oauthDao := dao.NewOAuthClientDao(database.GetDB())
	ctx := middleware.WrapCtx(c)

	client, err := oauthDao.GetByClientID(ctx, form.ClientID)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}
	if client.Status != 1 {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}

	// verify client secret
	// Note: in production, use constant-time comparison
	if form.ClientSecret != client.ClientSecret {
		response.Error(c, ecode.ErrOAuthInvalidClient)
		return
	}

	switch form.GrantType {
	case "authorization_code":
		h.handleAuthorizationCodeGrant(c, ctx, client, form)
	case "refresh_token":
		h.handleRefreshTokenGrant(c, ctx, client, form)
	default:
		response.Error(c, ecode.ErrOAuthUnsupportedGrant)
	}
}

func (h *usersHandler) handleAuthorizationCodeGrant(c *gin.Context, ctx context.Context, client *model.OAuthClient, form *types.OAuthTokenRequest) {
	if form.Code == "" || form.RedirectURI == "" {
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	// validate grant type
	if !oauth.ValidateGrantType(client, "authorization_code") {
		response.Error(c, ecode.ErrOAuthUnauthorizedClient)
		return
	}

	// validate redirect_uri
	if !oauth.ValidateRedirectURI(client, form.RedirectURI) {
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	// get and validate code
	ac, err := oauth.GetAuthorizationCode(ctx, form.Code)
	if err != nil {
		logger.Warn("GetAuthorizationCode error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// verify code belongs to this client
	if ac.ClientID != form.ClientID {
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// verify redirect_uri matches
	if ac.RedirectURI != form.RedirectURI {
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// validate PKCE
	if !oauth.ValidatePKCE(form.CodeVerifier, ac.PKCECodeChallenge, ac.PKCECodeChallengeMethod) {
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// generate tokens
	tokenResp, err := oauth.GenerateTokenPair(ac.UserID, ac.ClientID, ac.Scope)
	if err != nil {
		logger.Error("GenerateTokenPair error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.ErrOAuthServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"token_type":    tokenResp.TokenType,
		"expires_in":    tokenResp.ExpiresIn,
		"scope":         tokenResp.Scope,
	})
}

func (h *usersHandler) handleRefreshTokenGrant(c *gin.Context, ctx context.Context, client *model.OAuthClient, form *types.OAuthTokenRequest) {
	if form.RefreshToken == "" {
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	// validate grant type
	if !oauth.ValidateGrantType(client, "refresh_token") {
		response.Error(c, ecode.ErrOAuthUnauthorizedClient)
		return
	}

	// parse and validate refresh token
	claims, err := oauth.ParseAndValidateToken(form.RefreshToken)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// verify token type
	if claims.Type != "refresh" {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// verify client
	if claims.ClientID != form.ClientID {
		response.Error(c, ecode.ErrOAuthInvalidGrant)
		return
	}

	// check if revoked
	revoked, err := oauth.IsTokenRevoked(ctx, form.RefreshToken, "refresh")
	if err != nil {
		logger.Error("IsTokenRevoked error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.ErrOAuthServerError.ToHTTPCode())
		return
	}
	if revoked {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	// revoke old refresh token
	_ = oauth.RevokeToken(ctx, form.RefreshToken, "refresh")

	// generate new tokens
	tokenResp, err := oauth.GenerateTokenPair(claims.UserID, claims.ClientID, claims.Scope)
	if err != nil {
		logger.Error("GenerateTokenPair error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.ErrOAuthServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"token_type":    tokenResp.TokenType,
		"expires_in":    tokenResp.ExpiresIn,
		"scope":         tokenResp.Scope,
	})
}

// OAuthUserInfo userinfo endpoint
// @Summary OAuth userinfo endpoint
// @Description Get current user info using OAuth access token
// @Tags oauth
// @Accept json
// @Produce json
// @Success 200 {object} types.OAuthUserInfoReply{}
// @Router /api/v1/oauth/userinfo [get]
func (h *usersHandler) OAuthUserInfo(c *gin.Context) {
	claims, err := getOAuthClaimsFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrOAuthInvalidToken)
		return
	}

	ctx := middleware.WrapCtx(c)
	user, err := h.iDao.GetByID(ctx, claims.UserID)
	if err != nil {
		response.Error(c, ecode.ErrUserNotFound)
		return
	}

	response.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"phone":    user.Phone,
		"nickname": user.Nickname,
		"avatar":   user.Avatar,
	})
}

// OAuthRevoke revoke token endpoint
// @Summary OAuth revoke token endpoint
// @Description Revoke an access or refresh token
// @Tags oauth
// @Accept json
// @Produce json
// @Param data body types.OAuthRevokeRequest true "revoke request"
// @Success 200 {object} types.OAuthRevokeReply{}
// @Router /api/v1/oauth/revoke [post]
func (h *usersHandler) OAuthRevoke(c *gin.Context) {
	form := &types.OAuthRevokeRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}

	// determine token type
	tokenType := "access"
	if form.TokenTypeHint == "refresh_token" {
		tokenType = "refresh"
	}

	ctx := middleware.WrapCtx(c)
	if err := oauth.RevokeToken(ctx, form.Token, tokenType); err != nil {
		logger.Error("RevokeToken error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrOAuthRevokeFailed)
		return
	}

	response.Success(c)
}

// ==================== Helpers ====================

func oAuthRedirectError(c *gin.Context, redirectURI, errorCode, description, state string) {
	if redirectURI == "" {
		response.Error(c, ecode.ErrOAuthInvalidRequest)
		return
	}
	url := fmt.Sprintf("%s?error=%s&error_description=%s", redirectURI, errorCode, description)
	if state != "" {
		url += "&state=" + state
	}
	c.Redirect(http.StatusFound, url)
}

func getOAuthClaimsFromContext(c *gin.Context) (*jwt.OAuthClaims, error) {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return nil, ecode.ErrUnauthorized.Err()
	}
	token := authHeader[7:]
	claims, err := jwt.ParseOAuthToken(token)
	if err != nil {
		return nil, ecode.ErrUnauthorized.Err()
	}
	if claims.Type != "access" {
		return nil, ecode.ErrUnauthorized.Err()
	}
	return claims, nil
}
