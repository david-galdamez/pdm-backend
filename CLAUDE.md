# pdm-backend

Go/Gin/GORM/Postgres backend for a personal-finance mobile app. Originally a
university project written in Spanish; fully renamed to English and undergoing
a security/architecture overhaul on `refactor/backend-overhaul`. Not yet
released, so breaking changes to the API/schema are acceptable.

## Layout

`routes/` wire `middlewares/` → `controllers/` → `repositories/` → `models/`.
`services/` is a grab-bag (JWT, claims, query parsing). `websockets/` runs a
single broadcast goroutine reading from `websockets.BroadcastMessages`.
`internal/config` is the single source of env vars — never read `os.Getenv`
elsewhere.

## Commands

```bash
go build ./...
go vet ./...
gofmt -l .                          # must be empty
go run cmd/migrations/main.go       # AutoMigrate + seed lookup tables
go run cmd/resetdb/main.go          # drops every table, no confirmation
go run main.go
go test ./...                       # routes/authz_integration_test.go needs a
                                     # local Postgres at DATABASE_URL
```

## Refactor status

Tracking against the 9-phase plan from the overhaul. Phases 1–6 are done and
verified (rename, config, CORS, JWT, authorization, websockets). What's left:

### Known bugs (small, do these first)
- `controllers/saving_controller.go` `CreateSavingGoal`: compares
  `goalRequest.Month < currentMonth` without checking the year, so a goal for
  January of next year is wrongly rejected in the back half of this year.
- `controllers/saving_controller.go` `GetSavingsData`: hardcodes
  `year < 2025` instead of comparing against `time.Now().Year()`.

### Validation gaps
`TransactionRequest` has `binding:"required"` tags; almost nothing else does.
Add `binding:"required"` / `gt=0` / `email` as appropriate to:
- `RegisterRequest`, `LoginRequest` (user_controller.go) — empty
  email/password currently succeeds.
- `IncomeSourceRequest` — `amount: 0` or negative currently succeeds.
- `CategoryRequest`, `UpdateProfileRequest` — empty `name`/`email` currently
  succeeds.
- Sweep `subcategory_controller.go` / `shared_finance_controller.go` request
  structs for the same gap.

### Docker & CI (not started)
- `Dockerfile` (multi-stage, Go 1.24.1 per go.mod)
- `.dockerignore`
- `docker-compose.yml` (app + Postgres, for local dev)
- `.github/workflows/ci.yml` — build, vet, gofmt check, test
- `.env.example` — README documents the vars but there's no file to copy

### Server hardening (not started)
- `main.go` uses `r.Run()`; switch to an `http.Server` with
  `ReadTimeout`/`WriteTimeout`/`IdleTimeout` and graceful shutdown
  (`Shutdown(ctx)` on SIGINT/SIGTERM).
- No `/health` endpoint.
- No rate limiting on `/auth/login` or `/auth/register`.
- `cmd/resetdb/main.go` has no confirmation prompt or environment guard —
  running it points at whatever `DATABASE_URL` is currently set, including
  prod.

### Test coverage
Strong for authz (`routes/authz_integration_test.go`,
`middlewares/*_test.go`) and one GORM dry-run SQL test
(`repositories/shared_finance_repository_test.go`). Thin everywhere else — no
tests for dashboard aggregation math, savings goal computation, the three
transaction-creation branches, or websocket dispatch itself.

## Conventions established during the refactor

- Sentinel errors (`errors.Is`) over string-matching — see
  `repositories/shared_finance_repository.go`'s `ErrAlreadyMember` /
  `ErrInviteExpired`.
- Seeded lookup IDs and the `"Savings"` category name are named constants in
  `models/constants.go`, never bare numbers or string literals in queries.
- Finance-scoped endpoints never trust `?finance_id=` directly — they read it
  back from context via `services.FinanceId(c)` after
  `middlewares.FinanceAccess` (or `FinanceAccessFromParam`) has validated it.
- Admin-gated actions (`RequireFinanceAdmin`) return 404, not 403, so a
  non-admin member can't distinguish "not found" from "not allowed."
