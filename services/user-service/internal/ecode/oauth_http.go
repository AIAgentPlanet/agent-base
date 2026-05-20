package ecode

import (
	"github.com/go-dev-frame/sponge/pkg/errcode"
)

// oauth business-level http error codes.
// the oauthNO value range is 1~999, if the same error code is used, it will cause panic.
var (
	oauthNO       = 79
	oauthName     = "oauth"
	oauthBaseCode = errcode.HCode(oauthNO)

	ErrOAuthClientNotFound     = errcode.NewError(oauthBaseCode+1, "oauth client not found")
	ErrOAuthInvalidClient      = errcode.NewError(oauthBaseCode+2, "invalid client credentials")
	ErrOAuthInvalidGrant       = errcode.NewError(oauthBaseCode+3, "invalid grant")
	ErrOAuthInvalidRequest     = errcode.NewError(oauthBaseCode+4, "invalid request")
	ErrOAuthUnauthorizedClient = errcode.NewError(oauthBaseCode+5, "unauthorized client")
	ErrOAuthUnsupportedGrant   = errcode.NewError(oauthBaseCode+6, "unsupported grant type")
	ErrOAuthInvalidScope       = errcode.NewError(oauthBaseCode+7, "invalid scope")
	ErrOAuthInvalidToken       = errcode.NewError(oauthBaseCode+8, "invalid token")
	ErrOAuthTokenExpired       = errcode.NewError(oauthBaseCode+9, "token expired")
	ErrOAuthAccessDenied       = errcode.NewError(oauthBaseCode+10, "access denied")
	ErrOAuthServerError        = errcode.NewError(oauthBaseCode+11, "oauth server error")
	ErrOAuthClientExists       = errcode.NewError(oauthBaseCode+12, "oauth client already exists")
	ErrOAuthRevokeFailed       = errcode.NewError(oauthBaseCode+13, "failed to revoke token")
)
