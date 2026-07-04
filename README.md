# Base Go API

A production-ready Go (Golang) base project with a complete authentication
system: JWT auth, email verification, password reset, and email-based
two-factor authentication (2FA). It is a Go port of a FastAPI base template,
built with an idiomatic layered architecture.

## Features

- ✅ **Authentication**
  - JWT-based authentication
  - Email verification flow
  - Password reset flow
  - Email OTP two-factor authentication (2FA)
  - Account lockout after repeated failed logins

- ✅ **Security**
  - bcrypt password hashing
  - Per-IP rate limiting on auth endpoints
  - Configurable, enforced email verification
  - Optional forced 2FA for all users
  - Single-use, expiring tokens for verification & reset
  - Email-enumeration-safe password reset responses

- ✅ **Email**
  - Multiple providers (AWS SES, SMTP)
  - HTML + plain-text templates

- ✅ **Database**
  - PostgreSQL via GORM
  - SQL migrations embedded in the binary (golang-migrate)
  - Automatic DSN construction from components

- ✅ **Docker**
  - Multi-stage build
  - Compose files for development and production
  - Health checks

## Architecture

A clean, layered structure (handler → service → repository → model):

```
BaseGoAPI/
├── cmd/
│   └── server/          # application entry point (main.go)
├── internal/
│   ├── config/          # environment configuration
│   ├── apperror/        # typed HTTP errors (HTTPException equivalent)
│   ├── core/            # cross-cutting utilities
│   │   ├── security/    # bcrypt + secure token/OTP generation
│   │   ├── jwtutil/     # JWT create/parse
│   │   ├── email/       # SES / SMTP sender
│   │   ├── pagination/  # pagination helpers
│   │   └── validation/  # field validators
│   ├── model/           # GORM models
│   ├── repository/      # data-access layer (generic base + entities)
│   ├── dto/             # request/response payloads
│   ├── service/         # business logic
│   ├── middleware/      # auth, admin guard, rate limit, CORS, recovery
│   ├── handler/         # Gin HTTP handlers
│   ├── router/          # route registration
│   └── database/        # connection, migrations, seeding
├── migrations/          # SQL migration files (embedded)
├── Dockerfile
├── docker-compose.yml
├── docker-compose.prod.yml
├── Makefile
└── go.mod
```

### Layer mapping from the FastAPI original

| FastAPI                | Go                          |
| ---------------------- | --------------------------- |
| `app/core/config.py`   | `internal/config`           |
| `app/core/*`           | `internal/core/*`           |
| `app/model` (SQLAlchemy) | `internal/model` (GORM)   |
| `app/repository`       | `internal/repository`       |
| `app/schema` (Pydantic) | `internal/dto`             |
| `app/service`          | `internal/service`          |
| `app/router`           | `internal/handler` + `internal/router` |
| `app/service/deps.py`  | `internal/middleware`       |
| `HTTPException`         | `internal/apperror`         |
| Alembic                | `golang-migrate` (embedded) |

## Tech stack

- [Gin](https://github.com/gin-gonic/gin) — HTTP framework
- [GORM](https://gorm.io) — ORM
- [golang-migrate](https://github.com/golang-migrate/migrate) — migrations
- [golang-jwt](https://github.com/golang-jwt/jwt) — JWT
- [caarlos0/env](https://github.com/caarlos0/env) + [godotenv](https://github.com/joho/godotenv) — config
- AWS SDK v2 (SES) / [gomail](https://gopkg.in/gomail.v2) (SMTP) — email

## Prerequisites

- Go 1.23+
- PostgreSQL 15+
- Docker & Docker Compose (optional)

## Getting started

```bash
# 1. Configure
cp .env.example .env
# then edit .env (set JWT_SECRET_KEY, DB credentials, admin, email provider)

# 2. Install dependencies
go mod tidy

# 3. Run (migrations + admin seed run automatically at startup)
make run          # or: go run ./cmd/server
```

The API is served at `http://localhost:8000`.

## Docker

```bash
# Development
make docker-up            # docker compose up --build -d
make docker-down

# Production
make docker-prod          # docker compose -f docker-compose.prod.yml up --build -d
```

## API endpoints

### Authentication (`/v1/auth`)

| Method | Path               | Description                              |
| ------ | ------------------ | ---------------------------------------- |
| POST   | `/register`        | Register a new user (rate-limited)       |
| POST   | `/login`           | Login; returns a token or an OTP challenge (rate-limited) |
| POST   | `/verify-email?token=…` | Verify an email address             |
| POST   | `/forgot-password` | Request a password reset (rate-limited)  |
| POST   | `/reset-password`  | Reset a password with a token            |

### Admin (`/v1/admin`, requires a Bearer token + admin role)

| Method | Path          | Description                       |
| ------ | ------------- | --------------------------------- |
| GET    | `/users`      | List users (pagination/search/filter/sort) |
| GET    | `/users/:id`  | Get a user                        |
| POST   | `/users`      | Create a user                     |
| PUT    | `/users/:id`  | Update a user                     |
| DELETE | `/users/:id`  | Delete a user                     |

### Health

- `GET /v1/health` → `{"status":"ok"}`

## Authentication flow

The login response is polymorphic; branch on the `status` field:

- `status: "authenticated"` — includes `access_token` and `user`.
- `status: "otp_required"` — 2FA is on; an OTP was emailed. Re-POST to
  `/login` with the same credentials plus `otp_challenge_id` and `otp_code`.

2FA is triggered when `FORCE_TWO_FACTOR_AUTH=true` or the user has
`is_two_factor_enabled=true`.

## Configuration highlights

- `REQUIRE_EMAIL_VERIFICATION` — enforce verified email before login.
- `FORCE_TWO_FACTOR_AUTH` — require 2FA for every user on every login.
- `EMAIL_PROVIDER` — `aws_ses` or `smtp`.
- `CREATE_ADMIN_ON_STARTUP` — seed the bootstrap admin from `ADMIN_EMAIL` /
  `ADMIN_PASSWORD`.

See `.env.example` for the full list.

## Roles

The template ships with two generic roles defined in `internal/model/user.go`:

- `admin` — full access to the `/v1/admin` endpoints.
- `user` — the default role for self-service registrations.

Extend `validRoles` in `internal/service/user.go` (and the `Role*` constants) to
add your own.

## Security notes

1. **Passwords** — min 8 chars, must include upper/lower/digit; bcrypt-hashed.
2. **Account protection** — non-admin accounts lock after 3 failed logins;
   auth endpoints are rate-limited to 5 requests/minute/IP.
3. **Tokens** — cryptographically random, expiring, single-use, and cleaned up.
4. **Email privacy** — forgot-password returns the same response whether or not
   the account exists.

## Database migrations

Migrations live in `migrations/` as `NNNNNN_name.up.sql` / `.down.sql`, are
embedded into the binary via `go:embed`, and are applied automatically on
startup. To scaffold a new migration (needs the golang-migrate CLI):

```bash
make migrate-create name=add_widgets
```

## Development

```bash
make fmt     # go fmt ./...
make vet     # go vet ./...
make test    # go test ./...
make build   # build ./bin/server
```
