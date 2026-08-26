# 🏃‍♂️ Personal Finance Backend (Go)

[![CI](https://github.com/Befo0/pdm-backend/actions/workflows/ci.yml/badge.svg)](https://github.com/Befo0/pdm-backend/actions/workflows/ci.yml)
CI runs on every push/PR: `go vet` + `gofmt` checks, a full build (`go build ./...`), and the test suite against a Postgres service container.

Backend for a personal finance mobile app, built with Go, Gin and PostgreSQL.

## 📦 Requirements

- Go 1.25 or newer
- PostgreSQL ([Neon](https://neon.tech) works well for hosted environments)
- Git

## ⚙️ Environment variables

Copy `.env.example` to `.env` and fill it in:

```env
PORT=8000
DATABASE_URL=postgres://user:password@your-host.neon.tech:5432/your_database
JWT_SECRET=super_secret_key_at_least_32_characters
ENV=development
ALLOWED_ORIGINS=http://localhost:3000,https://app.example.com
```

`ALLOWED_ORIGINS` is the CORS allowlist: a comma-separated list of the exact web
origins (scheme + host + port) allowed to call the API from a browser. Native
mobile clients send no `Origin` header, so CORS does not apply to them. It is
required when `ENV=production`; in development it falls back to
`http://localhost:3000` and `http://localhost:5173`.

## Setup

1. Clone the repository

```bash
git clone git@github.com:Befo0/pdm-backend.git
cd pdm-backend
```

2. Install dependencies

```bash
go mod tidy
```

3. Run the database migrations

```bash
go run cmd/migrations/main.go
```

4. Start the server

```bash
go run main.go
```

The API listens on http://localhost:8000, with every route below mounted
under `/api`.

To drop every table and start over:

```bash
go run cmd/resetdb/main.go
```

This refuses to run when `ENV=production`, and otherwise prints the database
name and host it's about to wipe and requires you to type the database name
back to confirm.

## 🐳 Running with Docker

An alternative to the local Go setup above — brings up Postgres and the app
together, no local Go toolchain required.

```bash
docker compose up -d db             # start Postgres
docker compose run --rm migrate     # AutoMigrate + seed lookup tables
docker compose up -d app            # build the image and start the server
```

The API is then available on http://localhost:8080. `docker-compose.yml`
picks reasonable defaults for `JWT_SECRET`/`ALLOWED_ORIGINS`/Postgres
credentials for local dev (see the `x-app-env` block); override any of them by
exporting the corresponding environment variable before running `docker
compose`, e.g. `POSTGRES_PORT=15432` if port 5432 is already taken on your
machine.

To rebuild the app image after changing code:

```bash
docker compose up -d --build app
```

`docker compose down -v` stops everything and removes the Postgres volume.

## API

All routes except `GET /api/health`, `POST /api/auth/login` and
`POST /api/auth/register` require an `Authorization: Bearer <token>` header.
`/api/auth/login` and `/api/auth/register` are rate-limited per IP to guard
against brute-force attempts.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/health` | Liveness check, including a database ping |
| POST | `/api/auth/register` | Create an account and its personal finance |
| POST | `/api/auth/login` | Exchange credentials for a token |
| PATCH | `/api/users/me` | Update name and email |
| PATCH | `/api/users/me/password` | Change password |
| GET | `/api/finances/summary` | Dashboard totals for a month |
| GET | `/api/finances/breakdown` | Per-category budget vs. spend |
| GET | `/api/categories` | List expense categories |
| GET | `/api/categories/options` | Categories as selectable options |
| GET | `/api/categories/:id/breakdown` | Budget breakdown for one category |
| POST | `/api/categories` | Create a category |
| PATCH | `/api/categories/:id` | Rename a category |
| GET | `/api/subcategories` | List expense subcategories |
| GET | `/api/subcategories/budget-types` | Available budget types |
| GET | `/api/subcategories/:id` | Fetch one subcategory |
| POST | `/api/subcategories` | Create a subcategory |
| PUT | `/api/subcategories/:id` | Update a subcategory |
| GET | `/api/transactions` | Transactions for a month |
| GET | `/api/transactions/options` | Selectable income sources and subcategories |
| GET | `/api/transactions/:id` | Transaction details |
| POST | `/api/transactions` | Record a transaction |
| GET | `/api/income-sources` | List income sources |
| GET | `/api/income-sources/:id` | Fetch one income source |
| POST | `/api/income-sources` | Create an income source |
| PUT | `/api/income-sources/:id` | Update an income source |
| GET | `/api/savings` | Monthly goals and progress for a year |
| POST | `/api/savings/goals` | Create or update a monthly savings goal |
| GET | `/api/shared-finances` | Shared finances the user belongs to |
| GET | `/api/shared-finances/:id` | Shared finance details and members |
| POST | `/api/shared-finances` | Create a shared finance |
| POST | `/api/shared-finances/join` | Join using an invitation code |
| DELETE | `/api/shared-finances/:id/leave` | Leave a shared finance |
| DELETE | `/api/shared-finances/members/:id` | Remove a member |
| POST | `/api/invitations/:id` | Create an invitation code for a finance |
| GET | `/api/ws/finances/:id` | WebSocket stream of finance events |

Endpoints that operate on a finance accept an optional `?finance_id=` query
parameter; without it they fall back to the caller's personal finance.
Month-scoped endpoints take `?month=` and `?year=`.

## Architecture & decisions

Gin + GORM were chosen early on for development speed — this started as a
university project on a tight deadline, so a batteries-included router and
ORM mattered more than raw performance. Requests flow through a fixed
pipeline: middleware → controller → repository → model.

```
Client → middlewares (auth, finance access, rate limit)
       → controllers (bind + validate request)
       → repositories (GORM queries, one DB transaction per write)
       → models (Postgres)
```

Once the app moved past the university project stage, the backend went
through a security/architecture hardening pass:

- **Rate limiting** on `/auth/login` and `/auth/register` — these were
  previously unprotected, leaving them open to brute-force credential
  guessing.
- **Graceful shutdown** — the server used to drop in-flight requests and
  open websocket connections on deploy/restart; it now stops accepting new
  connections and closes existing ones cleanly before exiting.
- **Automated tests** (`controllers`, `repositories`, `routes`, `middlewares`)
  — the original codebase had none, so correctness relied on manual
  QA. Coverage was added for authorization, repository aggregation math, and
  request validation, run automatically in CI on every push.

## Operational notes

- The HTTP server shuts down gracefully on `SIGINT`/`SIGTERM`: it stops
  accepting new connections, and closes every open websocket with a proper
  close frame before exiting (bounded by a 5-second timeout).
- `GET /api/health` reports `200` only if the database responds to a ping.
