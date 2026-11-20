// Command migrate generates versioned SQL migrations (Atlas format) from
// the Ent schema. Production applies these files with the Atlas CLI rather
// than calling Schema.Create at startup.
//
//	go run ./cmd/migrate -dev-url 'postgres://postgres:postgres@localhost:5432/streakit_atlas_dev?sslmode=disable' add_venues
//
// The dev database must be empty: it is used to replay existing migrations
// and compute the diff, never to store data.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	atlas "ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/niteshs1/streakit-api/ent"
)

func main() {
	devURL := flag.String("dev-url", os.Getenv("ATLAS_DEV_URL"), "URL of an EMPTY Postgres database used to compute diffs")
	dirPath := flag.String("dir", "migrations", "migration directory")
	flag.Parse()

	if flag.NArg() != 1 || *devURL == "" {
		fmt.Fprintln(os.Stderr, "usage: migrate -dev-url <postgres-url> <migration_name>")
		os.Exit(2)
	}

	if err := run(context.Background(), *devURL, *dirPath, flag.Arg(0)); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, devURL, dirPath, name string) error {
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}
	dir, err := atlas.NewLocalDir(dirPath)
	if err != nil {
		return fmt.Errorf("open migration dir: %w", err)
	}

	db, err := sql.Open("pgx", devURL)
	if err != nil {
		return err
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer client.Close()

	return client.Schema.NamedDiff(ctx, name,
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(atlas.DefaultFormatter),
	)
}
