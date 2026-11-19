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

func NewPostgresClient(
	ctx context.Context,
	cfg config.DatabaseConfig,
) (*ent.Client, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres driver: %w", err)
	}

	// Use the pgx driver for the connection, but tell ent it's Postgres.
	drv := entsql.OpenDB(dialect.Postgres, db)

	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnectionMaxLifetime)

	client := ent.NewClient(ent.Driver(drv))

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	return client, nil
}
