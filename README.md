# StreakIt API

Staff (super admin / admin) backend — Go, Gin, Ent, Postgres. Design: [docs/staff-foundation-api.md](docs/staff-foundation-api.md).

## Quick start

```sh
cp .env.example .env        # set JWT_SECRET and SUPERADMIN_* (see comments)
make db-up                  # Postgres via docker compose
make run                    # migrates (dev), bootstraps the super admin, serves :8080
```

Swagger UI: <http://localhost:8080/swagger/index.html> (on in every environment,
including production; set `SWAGGER_ENABLED=false` to hide it).
Call `POST /auth/login` first — the session lives in HttpOnly cookies, so the
UI's "Try it out" calls are authenticated automatically.

### Upgrading an existing local DB

The first version used integer user IDs; this slice uses UUIDs and
`Schema.Create` cannot change a column type. Reset your **local** DB once
(this deletes all local data):

```sh
make db-reset && make run
```

## Layout

```text
cmd/api              entrypoint (config → DB → server.New → listen)
cmd/migrate          generates versioned SQL migrations from the Ent schema
ent/schema           Ent schemas (only entrepo + database import ent)
internal/server      wiring of repos → services → handlers → router
internal/routes      URL map + per-route auth/role middleware
internal/handler     bind → service → envelope (no business rules)
internal/service     business rules, transactions, audit
internal/repository  interfaces + TxManager; entrepo (Ent) and memrepo (tests)
internal/dto         request/response shapes
internal/domain      core types
internal/apperror    typed errors with stable codes
internal/response    {success, message, data|error} envelope
internal/auth        bcrypt, HS256 JWT, refresh tokens
migrations/          Atlas-format versioned migrations
docs/swagger         generated OpenAPI (make swagger)
```

## Common tasks

| Task | Command |
| --- | --- |
| Unit + handler tests | `make test` |
| Integration tests (real Postgres) | `make test-integration` |
| Regenerate Ent code | `make generate` |
| Regenerate Swagger | `make swagger` |
| New migration after schema change | `make migrate-diff name=<name>` |
| Apply migrations (staging/prod, needs Atlas CLI) | `make migrate-apply DATABASE_URL=...` |

In staging/production keep `DATABASE_AUTO_MIGRATE=false` (the default there)
and apply `migrations/` with Atlas.

## API conventions

- Base path `/api/v1`. Every response: `{success, message, data}` or
  `{success:false, message, error:{code, fields?}}`.
- Mutating requests must send `Content-Type: application/json` (CSRF guard),
  even when they have no body (e.g. `POST /events/:id/publish`).
- `401 token_expired` → call `POST /auth/refresh`, then retry.
- `PATCH` on venues and events requires the current `version`; a stale
  version returns `409 conflict`.
- Status changes use `POST /:id/<action>` (block, retire, hide, publish, …).
