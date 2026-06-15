package initial

import (
	"strconv"

	"github.com/go-dev-frame/sponge/pkg/app"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/pkg/ath"
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
	if !db.Migrator().HasTable(&model.ATHAuditRecord{}) {
		if err := db.Migrator().CreateTable(&model.ATHAuditRecord{}); err != nil {
			panic("create ath_audit_records table error: " + err.Error())
		}
	}
	if !db.Migrator().HasTable(&model.ATHAuditOutbox{}) {
		if err := db.Migrator().CreateTable(&model.ATHAuditOutbox{}); err != nil {
			panic("create ath_audit_outboxes table error: " + err.Error())
		}
	}
	const auditImmutabilitySQL = `
CREATE OR REPLACE FUNCTION prevent_ath_audit_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'ath_audit_records is append-only';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'ath_audit_records_immutable'
    ) THEN
        CREATE TRIGGER ath_audit_records_immutable
        BEFORE UPDATE OR DELETE ON ath_audit_records
        FOR EACH ROW EXECUTE FUNCTION prevent_ath_audit_mutation();
    END IF;
END;
$$;`
	if err := db.Exec(auditImmutabilitySQL).Error; err != nil {
		panic("create ath audit immutability trigger error: " + err.Error())
	}
	if worker := ath.DefaultAnchorWorker(); worker != nil {
		worker.Start()
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
