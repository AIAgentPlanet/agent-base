package routers

import (
	"github.com/gin-gonic/gin"

	"agent-base/services/user-service/internal/handler"
)

func init() {
	publicRouterFns = append(publicRouterFns, func(r *gin.Engine) {
		r.GET("/.well-known/ath.json", handler.NewUsersHandler().ATHDiscovery)
		r.GET("/.well-known/did.json", handler.NewUsersHandler().ATHServerIdentity)
		r.GET("/.well-known/ath-audit-head.json", handler.NewUsersHandler().ATHAuditHead)
	})
	apiV1RouterFns = append(apiV1RouterFns, func(group *gin.RouterGroup) {
		athRouter(group, handler.NewUsersHandler())
	})
}

func athRouter(group *gin.RouterGroup, h handler.UsersHandler) {
	// ATH discovery document (public)
	group.GET("/.well-known/ath.json", h.ATHDiscovery)

	// ATH endpoints (public, authenticated via agent attestation or client_secret)
	athGroup := group.Group("/ath")
	athGroup.POST("/agents/register", h.ATHRegister)
	athGroup.GET("/agents/:clientId", h.ATHAgentStatus)
	athGroup.POST("/handshakes", h.ATHStartHandshake)
	athGroup.POST("/handshakes/:handshakeId/proof", h.ATHCompleteHandshake)
	athGroup.GET("/handshakes/:handshakeId", h.ATHHandshakeStatus)
	athGroup.POST("/audit/query", h.ATHAuditQuery)
	athGroup.POST("/audit/verify", h.ATHAuditVerify)
	athGroup.POST("/audit/anchor/status", h.ATHAnchorStatus)
	athGroup.POST("/audit/anchor/retry", h.ATHAnchorRetry)
	athGroup.POST("/authorize", h.ATHAuthorize)
	athGroup.POST("/token", h.ATHToken)
	athGroup.POST("/revoke", h.ATHRevoke)
	athGroup.POST("/proxy", h.ATHProxy)
}
