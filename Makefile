.PHONY: hooks run build test vet fmt tidy generate db-up db-down db-logs up down docker-build clean

APP_NAME=streakit-api

run:
	go run ./cmd/api

build:
	go build -trimpath -o bin/api ./cmd/api

test:
	go test -race -v ./...

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal ./ent/schema

tidy:
	go mod tidy

generate:
	go tool ent generate ./ent/schema

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

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
