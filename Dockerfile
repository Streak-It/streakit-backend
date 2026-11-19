# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- Runtime stage ----
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app -H -s /sbin/nologin app

WORKDIR /app

COPY --from=builder --chown=app:app /out/api ./api

USER app

ENV APP_ENV=production \
    APP_PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${APP_PORT}/health" >/dev/null || exit 1

# Exec form => the binary is PID 1 and receives SIGTERM directly.
ENTRYPOINT ["./api"]
