# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" \
    -o /app/bin/api ./cmd/api

# Runtime stage
FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates \
    && addgroup -S appgroup \
    && adduser -S appuser -G appgroup

COPY --from=builder /app/bin/api ./api

USER appuser

EXPOSE 8080

CMD ["./api"]