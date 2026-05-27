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

	// create database tables if not exist
	db := database.GetDB()
	if !db.Migrator().HasTable(&model.Users{}) {
		if err := db.Migrator().CreateTable(&model.Users{}); err != nil {
			panic("create users table error: " + err.Error())
		}
	}
	if !db.Migrator().HasTable(&model.OAuthClient{}) {
		if err := db.Migrator().CreateTable(&model.OAuthClient{}); err != nil {
			panic("create oauth_clients table error: " + err.Error())
		}
	}
	if !db.Migrator().HasTable(&model.ATHAgent{}) {
		if err := db.Migrator().CreateTable(&model.ATHAgent{}); err != nil {
			panic("create ath_agents table error: " + err.Error())
		}
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
