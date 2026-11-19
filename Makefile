.PHONY: run build test fmt tidy generate db-up db-down db-logs clean

APP_NAME=go-backend-starter

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test -v ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

generate:
	go run entgo.io/ent/cmd/ent generate ./ent/schema

db-up:
	docker compose up -d

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

clean:
	rm -rf bin/