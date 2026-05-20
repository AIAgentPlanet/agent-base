package initial

import (
	"strconv"

	"github.com/go-dev-frame/sponge/pkg/app"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/server"
)

// CreateServices create http service
func CreateServices() []app.IServer {
	var cfg = config.Get()
	var servers []app.IServer

	// auto migrate database tables
	db := database.GetDB()
	if err := db.AutoMigrate(&model.Users{}, &model.OAuthClient{}); err != nil {
		panic("auto migrate error: " + err.Error())
	}

	// create a http service
	httpAddr := ":" + strconv.Itoa(cfg.HTTP.Port)
	httpServer := server.NewHTTPServer(httpAddr,
		server.WithHTTPIsProd(cfg.App.Env == "prod"),
		server.WithHTTPTLS(cfg.HTTP.TLS),
	)
	servers = append(servers, httpServer)

	return servers
}
