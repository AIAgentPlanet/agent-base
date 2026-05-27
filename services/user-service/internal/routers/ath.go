package routers

import (
	"github.com/gin-gonic/gin"

	"agent-base/services/user-service/internal/handler"
)

func init() {
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
	athGroup.POST("/authorize", h.ATHAuthorize)
	athGroup.POST("/token", h.ATHToken)
	athGroup.POST("/revoke", h.ATHRevoke)
	athGroup.POST("/proxy", h.ATHProxy)
}
