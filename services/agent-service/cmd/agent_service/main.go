package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"agent-base/services/agent-service/internal/api"
	"agent-base/services/agent-service/internal/athclient"
	"agent-base/services/agent-service/internal/integrity"
	"agent-base/services/agent-service/internal/runtime"
	"agent-base/services/agent-service/internal/token"
)

func main() {
	addr := os.Getenv("AGENT_SERVICE_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	store := runtime.NewMemoryStore()
	handler := api.NewHandler(
		store,
		api.WithATHAuditClient(athclient.New(athclient.Config{
			BaseURL:      os.Getenv("ATH_AUDIT_BASE_URL"),
			ClientID:     os.Getenv("ATH_AUDIT_CLIENT_ID"),
			ClientSecret: os.Getenv("ATH_AUDIT_CLIENT_SECRET"),
			Timeout:      5 * time.Second,
		})),
		api.WithATHTokenVerifier(token.NewATHJWTVerifier(token.Config{
			Secret: os.Getenv("ATH_JWT_SECRET"),
			Issuer: os.Getenv("ATH_JWT_ISSUER"),
		})),
		api.WithATHIntegrityVerifier(integrity.New(integrity.Config{
			Secret: os.Getenv("ATH_INTEGRITY_SECRET"),
		})),
	)

	log.Printf("agent-service listening on %s", addr)
	if err := http.ListenAndServe(addr, handler.Routes()); err != nil {
		log.Fatal(err)
	}
}
