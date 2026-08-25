# pdm-backend

Go/Gin/GORM/Postgres backend for a personal-finance mobile app. Originally a
university project written in Spanish; fully renamed to English and undergoing
a security/architecture overhaul on `refactor/backend-overhaul`. Not yet
released, so breaking changes to the API/schema are acceptable.

## Layout

`routes/` wire `middlewares/` → `controllers/` → `repositories/` → `models/`.
Every `*Router` func takes a `*gin.RouterGroup`, not `*gin.Engine` — `main.go`
mounts them all under `api := r.Group("/api")`. `services/` is a grab-bag (JWT,
claims, query parsing). `websockets/` runs a single broadcast goroutine reading
from `websockets.BroadcastMessages`. `internal/config` is the single source of
env vars — never read `os.Getenv` elsewhere.

## Commands

```bash
go build ./...
go vet ./...
gofmt -l .                          # must be empty
go run cmd/migrations/main.go       # AutoMigrate + seed lookup tables
go run cmd/resetdb/main.go          # drops every table; refuses on ENV=production,
                                     # asks you to retype the db name to confirm
go run main.go
go test ./...                       # routes/authz_integration_test.go needs a
                                     # local Postgres at DATABASE_URL
```

## Refactor status

Tracking against the 9-phase plan from the overhaul. Phases 1–6 are done and
verified (rename, config, CORS, JWT, authorization, websockets). What's left:

### Known bugs — done
Both savings date bugs are fixed and covered by
`controllers/saving_controller_test.go`.

### Validation gaps — done
Every request struct now carries `binding` rules, and
`controllers/binding_tags_test.go` guards them. Two traps worth remembering:
- A space after a comma (`binding:"required, gt=0"`) makes validator look up a
  rule named `" gt"`, which **panics** at request time. gofmt and go vet both
  pass it. The test scans every request struct for padded rules.
- `required` rejects a numeric zero, so `required,gte=0` can never accept 0.
  Use `gte=0` alone when zero is a legal value.

### Repository correctness — done
Swept in the same pass, with integration coverage in `repositories/*_test.go`:
- Aggregations never join a to-many table before `SUM`: joining transactions
  before grouping multiplied budgets by the transaction count. Spend is a
  correlated subquery instead (`GetCategoriesData`, `GetDataSummary`).
- GORM only applies `deleted_at IS NULL` to the **primary** model, never to
  `Joins(...)`. Every join condition spells the filter out by hand.
- Check `tx.Error` **before** `tx.RowsAffected == 0`: a failed query also
  reports zero rows, which turns a 500 into a silent 404.
- `Scan` into a struct returns zero values and a nil error when nothing
  matched; lookups that must find something check `RowsAffected` and return
  `gorm.ErrRecordNotFound`.
- Counters are incremented in the `UPDATE` (`gorm.Expr("amount + ?")`), never
  read-modify-written in Go. Partial unique indexes on
  `monthly_goals`/`monthly_savings`/`shared_finances` back the upserts; the
  insert path falls back to an update on `IsUniqueViolation`.

### Docker & CI (not started)
- `Dockerfile` (multi-stage, Go 1.25.0 per go.mod)
- `.dockerignore`
- `docker-compose.yml` (app + Postgres, for local dev)
- `.github/workflows/ci.yml` — build, vet, gofmt check, test
- `.env.example` — README documents the vars but there's no file to copy

### Server hardening — done
`main.go` runs an `http.Server` with `ReadTimeout`/`WriteTimeout`/`IdleTimeout`,
graceful shutdown on SIGINT/SIGTERM, a `GET /api/health` endpoint (200 only if
`sqlDB.Ping()` succeeds), and per-IP rate limiting on `/auth/login` and
`/auth/register` (`middlewares/rate_limiter.go`). `cmd/resetdb/main.go` refuses
to run when `ENV=production` and otherwise requires retyping the database name
parsed from `DATABASE_URL` back to confirm. Traps worth remembering:
- `Upgrade()` hijacks the connection, and `net/http` stops tracking a hijacked
  connection immediately — `Server.Shutdown()` never waits for it and never
  cancels its request context. Waiting on `c.Done()` inside a websocket handler
  to notice shutdown does nothing; the handler needs its own shutdown signal
  (a channel `close()`, not a send — a send only wakes one of N waiting
  connections, and blocks forever if none are currently open) broadcast from
  `main.go`, and a read deadline set at that point (not at connect time) so a
  non-cooperating client can't block the handler forever.
- `time.Time.Add` returns a new value; it does not mutate the receiver. The
  rate limiter's sliding expiry must be reassigned
  (`cl.expiry = time.Now().Add(...)`), not called and discarded.
- The rate limiter's cleanup goroutine must hold the map mutex for the whole
  scan, not just the deletes — ranging over a map while another goroutine
  writes to it under its own lock is a data race even though both sides use
  the same `sync.Mutex` in isolation.

### Test coverage
Strong for authz (`routes/authz_integration_test.go`, `middlewares/*_test.go`),
for repository math and concurrency (`repositories/*_test.go`), and for request
validation (`controllers/*_test.go`).

`repositories/main_test.go` provisions its own `finance_app_repo_test` database
(the `routes` suite truncates tables, and package test binaries run
concurrently). Cases call `requireDB(t)`, which **skips** when no local
Postgres answers, so `go test ./...` works without one — unlike the `routes`
suite, which hard-fails. Override the server with `TEST_POSTGRES_DSN` (a format
string with one `%s` for the database name).

Still untested: websocket dispatch itself, the graceful-shutdown close
handshake, the rate limiter (`middlewares/rate_limiter.go` has no test file),
and the transaction-creation branches end to end through the router.

## Conventions established during the refactor

- Sentinel errors (`errors.Is`) over string-matching — see
  `repositories/shared_finance_repository.go`'s `ErrAlreadyMember` /
  `ErrInviteExpired` / `ErrAdminCannotLeave`. For driver errors use
  `repositories.IsUniqueViolation` (SQLSTATE 23505), never `strings.Contains`
  on the message.
- Client-supplied ids are re-read scoped to the authorized finance before they
  are written onto a record — `TransactionRepo.GetIds(subcategoryId,
  financeId)`, `IncomeSourceBelongsToFinance`. Passing the id straight through
  lets one finance write against another's rows.
- Writes that belong together go in one `db.Transaction` — see
  `CreateTransactionWithSaving`.
- Seeded lookup IDs and the `"Savings"` category name are named constants in
  `models/constants.go`, never bare numbers or string literals in queries.
- Finance-scoped endpoints never trust `?finance_id=` directly — they read it
  back from context via `services.FinanceId(c)` after
  `middlewares.FinanceAccess` (or `FinanceAccessFromParam`) has validated it.
- Admin-gated actions (`RequireFinanceAdmin`) return 404, not 403, so a
  non-admin member can't distinguish "not found" from "not allowed."
