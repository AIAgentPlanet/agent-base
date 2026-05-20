package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/go-dev-frame/sponge/pkg/gin/middleware"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/handler"
)

func init() {
	apiV1RouterFns = append(apiV1RouterFns, func(group *gin.RouterGroup) {
		usersRouter(group, handler.NewUsersHandler())
	})
}

func usersRouter(group *gin.RouterGroup, h handler.UsersHandler) {
	g := group.Group("/users")

	// Public routes (no authentication required)
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/reset-code", h.SendResetCode)
	g.POST("/reset-password", h.ResetPassword)

	// Protected routes (JWT authentication required)
	authGroup := g.Group("")
	authGroup.Use(middleware.Auth(middleware.WithSignKey([]byte(config.Get().JWT.Secret))))

	authGroup.GET("/profile", h.GetProfile)
	authGroup.PUT("/profile", h.UpdateProfile)

	// CRUD routes (JWT authentication required)
	authGroup.POST("/", h.Create)
	authGroup.DELETE("/:id", h.DeleteByID)
	authGroup.PUT("/:id", h.UpdateByID)
	authGroup.GET("/:id", h.GetByID)
	authGroup.POST("/list", h.List)
	authGroup.POST("/delete/ids", h.DeleteByIDs)
	authGroup.POST("/condition", h.GetByCondition)
	authGroup.POST("/list/ids", h.ListByIDs)
	authGroup.GET("/list", h.ListByLastID)
}
