.PHONY: hooks run build test test-integration vet fmt tidy generate swagger db-reset migrate-diff migrate-apply db-up db-down db-logs up down docker-build clean

APP_NAME=streakit-api

run:
	go run ./cmd/api

build:
	go build -trimpath -o bin/api ./cmd/api

test:
	go test -race -v ./...

# Integration tests against the compose Postgres (each test uses its own schema).
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/streakit_test?sslmode=disable
test-integration: db-up
	@docker compose exec -T postgres psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='streakit_test'" | grep -q 1 \
		|| docker compose exec -T postgres createdb -U postgres streakit_test
	STREAKIT_TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -race -count=1 -v ./internal/integration/...

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal ./ent/schema

tidy:
	go mod tidy

generate:
	go tool ent generate --feature sql/lock,sql/versioned-migration,sql/execquery ./ent/schema

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

# DESTRUCTIVE: drops and recreates the local dev database. Needed once after
# the int -> UUID id switch, because Schema.Create cannot alter id types.
DATABASE_NAME ?= streakit_api
db-reset: db-up
	docker compose exec -T postgres dropdb -U postgres --if-exists --force $(DATABASE_NAME)
	docker compose exec -T postgres createdb -U postgres $(DATABASE_NAME)
	@echo "database recreated; run 'make run' (DATABASE_AUTO_MIGRATE=true) to create tables"

# Generate a versioned migration from the Ent schema: make migrate-diff name=add_x
# Uses a throwaway empty database as the Atlas dev database.
ATLAS_DEV_DB = streakit_atlas_dev
migrate-diff: db-up
	@test -n "$(name)" || (echo "usage: make migrate-diff name=<migration_name>" && exit 1)
	docker compose exec -T postgres dropdb -U postgres --if-exists $(ATLAS_DEV_DB)
	docker compose exec -T postgres createdb -U postgres $(ATLAS_DEV_DB)
	go run ./cmd/migrate -dev-url 'postgres://postgres:postgres@localhost:5432/$(ATLAS_DEV_DB)?sslmode=disable' $(name)
	docker compose exec -T postgres dropdb -U postgres $(ATLAS_DEV_DB)

# Apply versioned migrations (requires the Atlas CLI: https://atlasgo.io).
# Use this for staging/production instead of DATABASE_AUTO_MIGRATE.
migrate-apply:
	@test -n "$(DATABASE_URL)" || (echo "set DATABASE_URL=postgres://..." && exit 1)
	atlas migrate apply --dir file://migrations --url '$(DATABASE_URL)'

up:
	docker compose --profile app up -d --build

down:
	docker compose --profile app down

docker-build:
	docker build -t $(APP_NAME):latest .

clean:
	rm -rf bin/

# Install git hooks (Conventional Commits check). Run once after cloning.
hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo "git hooks installed"

# Regenerate OpenAPI docs into docs/swagger (served at /swagger in development).
swagger:
	go tool swag fmt -d ./cmd/api,./internal/handler
	go tool swag init -d ./cmd/api,./internal/handler,./internal/dto,./internal/response,./internal/apperror,./internal/domain -g main.go -o docs/swagger --outputTypes go,json,yaml --parseInternal
