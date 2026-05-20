package database

import (
	"time"

	"github.com/go-dev-frame/sponge/pkg/sgorm"
	"github.com/go-dev-frame/sponge/pkg/sgorm/postgresql"

	"agent-base/services/user-service/internal/config"
)

// InitPostgresql connect postgresql
func InitPostgresql() *sgorm.DB {
	cfg := config.Get().Database.Postgresql
	db, err := postgresql.Init(
		cfg.Dsn,
		postgresql.WithMaxIdleConns(cfg.MaxIdleConns),
		postgresql.WithMaxOpenConns(cfg.MaxOpenConns),
		postgresql.WithConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime)*time.Minute),
	)
	if err != nil {
		panic("InitPostgresql error: " + err.Error())
	}
	if cfg.EnableLog {
		db = db.Debug()
	}
	return db
}
