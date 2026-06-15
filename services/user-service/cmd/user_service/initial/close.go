package initial

import (
	"context"
	"time"

	"github.com/go-dev-frame/sponge/pkg/app"
	"github.com/go-dev-frame/sponge/pkg/logger"
	"github.com/go-dev-frame/sponge/pkg/tracer"

	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/pkg/ath"
)

// Close releasing resources after service exit
func Close(servers []app.IServer) []app.Close {
	var closes []app.Close

	// close server
	for _, s := range servers {
		closes = append(closes, s.Stop)
	}

	// stop anchor delivery before closing the database
	if worker := ath.DefaultAnchorWorker(); worker != nil {
		closes = append(closes, worker.Close)
	}

	// close database
	closes = append(closes, func() error {
		return database.CloseDB()
	})

	// close redis
	if config.Get().App.CacheType == "redis" {
		closes = append(closes, func() error {
			return database.CloseRedis()
		})
	}

	// close tracing
	if config.Get().App.EnableTrace {
		closes = append(closes, func() error {
			ctx, _ := context.WithTimeout(context.Background(), 2*time.Second) //nolint
			return tracer.Close(ctx)
		})
	}

	// close logger
	closes = append(closes, func() error {
		return logger.Sync()
	})

	return closes
}
