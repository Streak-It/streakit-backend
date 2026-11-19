package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/internal/config"
)

// DB bundles the ent client with the underlying *sql.DB so callers can
// ping the pool (readiness checks) and inspect pool stats.
type DB struct {
	Ent *ent.Client
	SQL *sql.DB
}

func NewPostgres(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres driver: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnectionMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnectionMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	// Use the pgx driver for the connection, but tell ent it's Postgres.
	drv := entsql.OpenDB(dialect.Postgres, db)

	return &DB{
		Ent: ent.NewClient(ent.Driver(drv)),
		SQL: db,
	}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.SQL.PingContext(ctx)
}

// Close closes the ent client, which also closes the underlying pool.
func (d *DB) Close() error {
	return d.Ent.Close()
}
