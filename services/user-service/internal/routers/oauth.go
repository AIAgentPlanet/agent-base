package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/go-dev-frame/sponge/pkg/gin/middleware"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/handler"
)

func init() {
	apiV1RouterFns = append(apiV1RouterFns, func(group *gin.RouterGroup) {
		oauthRouter(group, handler.NewUsersHandler())
	})
}

func oauthRouter(group *gin.RouterGroup, h handler.UsersHandler) {
	g := group.Group("/oauth")

	// Public OAuth 2.0 endpoints
	g.POST("/token", h.OAuthToken)
	g.GET("/userinfo", h.OAuthUserInfo)
	g.POST("/revoke", h.OAuthRevoke)

	// Authorization endpoint (requires user authentication)
	authGroup := g.Group("")
	authGroup.Use(middleware.Auth(middleware.WithSignKey([]byte(config.Get().JWT.Secret))))
	authGroup.GET("/authorize", h.OAuthAuthorize)

	// Client management (requires user authentication)
	clientGroup := g.Group("/clients")
	clientGroup.Use(middleware.Auth(middleware.WithSignKey([]byte(config.Get().JWT.Secret))))
	clientGroup.POST("", h.CreateOAuthClient)
	clientGroup.POST("/list", h.ListOAuthClients)
	clientGroup.PUT("/:id", h.UpdateOAuthClient)
	clientGroup.DELETE("/:id", h.DeleteOAuthClient)
}
