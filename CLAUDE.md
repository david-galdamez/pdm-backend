# pdm-backend

Go/Gin/GORM/Postgres backend for a personal-finance mobile app. Originally a
university project written in Spanish; fully renamed to English and undergoing
a security/architecture overhaul on `refactor/backend-overhaul`. Not yet
released, so breaking changes to the API/schema are acceptable.

## Layout

`routes/` wire `middlewares/` → `controllers/` → `repositories/` → `models/`.
Every `*Router` func takes a `*gin.RouterGroup`, not `*gin.Engine` — `main.go`
mounts them all under `api := r.Group("/api")`. `services/` is a grab-bag (JWT,
claims, query parsing). `websockets/` runs a single broadcast goroutine that
subscribes to an `events.Subscriber` (`HandleBroadCast(ctx, sub)`); producers
call `events.Publisher.Publish`. Both interfaces live in `events/`, and
`events.MemoryBroker` is the in-process implementation of both (a buffered Go
channel), wired in `main.go`. `internal/config` is the single source of env
vars — never read `os.Getenv` elsewhere.

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

docker compose up -d db             # local dev: Postgres only
docker compose run --rm migrate     # local dev: AutoMigrate + seed, against the db above
docker compose up -d app            # local dev: builds Dockerfile, runs the server
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

### Docker & CI — done
`Dockerfile` is multi-stage: a `golang:1.25.0-alpine` builder compiles
`server`, `migrate`, and `resetdb` (`CGO_ENABLED=0` — pgx is pure Go, so the
binaries are static) into a final `alpine:3.20` image running as a non-root
user, with a `HEALTHCHECK` against `/api/health`. `.dockerignore` keeps `.git`,
`.env`, and docs out of the build context — without it a local `.env` would
have been copied into the image by `COPY . .`.

`docker-compose.yml` is local-dev only (`db` + `app` + a `migrate` one-off
service under `profiles: ["tools"]`, run via `docker compose run --rm
migrate`); it is not used in CI. `.github/workflows/ci.yml` runs a `build` job
(build/vet/gofmt) and a separate `test` job that starts Postgres via GitHub
Actions' own `services:` block (not Compose) seeded as
`finance_app_test`/`postgres`/`analissa` to match the hardcoded DSN in
`routes/authz_integration_test.go`; `go test ./...` then runs directly on the
runner against `localhost:5432`. `.env.example` documents the same vars as the
README.

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

`events/memory_broker_test.go` covers `MemoryBroker` with no DB: delivery and
event-set shape (saving adds `finance_savings`), FIFO order, `Subscribe`
returning on `ctx` cancel, and `Publish` never blocking once the buffer is
full.

Still untested: websocket dispatch itself, the graceful-shutdown close
handshake, the rate limiter (`middlewares/rate_limiter.go` has no test file),
and the transaction-creation branches end to end through the router.

## Messaging & scaling plan — decided, publisher interface extracted

The app currently runs as a single instance and broadcasts websocket events
over a buffered Go channel owned by `events.MemoryBroker`, behind the
`events.Publisher` interface (`controllers/transaction_controller.go` holds an
`events.Publisher`, not the channel). That is the correct design at one
instance and is **not** a stopgap: no network hop, no serialization, ordering
for free, no extra failure mode. Only the transport is now swappable.

### The two use cases have different justifications
Keeping them separate is what lets one be built now and the other deferred
without it being a compromise:
- **Websockets over a broker** is justified *only* by instance count. At one
  instance a broker would be strictly worse in every dimension.
- **Durable background work** (email) is justified by durability and by
  getting slow work off the request path. Both hold at one instance, ten, or
  a hundred — instance count never enters the argument.

### Decision: RabbitMQ now for email, Redis later for fan-out
- **RabbitMQ** for the email/notification worker, as its own binary added to
  the existing multi-stage `Dockerfile` alongside `server`/`migrate`/`resetdb`.
  Email is slow, must not be lost, fails transiently, and needs a dead-letter
  queue for permanent failures — exactly RabbitMQ's feature set. A `jobs`
  table in Postgres or Redis Streams would also work; RabbitMQ was chosen
  deliberately for its retry/DLQ tooling and to learn the model properly.
- **Redis Pub/Sub + a Redis rate limiter** only when a second instance is
  actually needed. Not before.
- **Not Kafka.** Nothing here is worth replaying, and partitions/offsets/
  consumer groups buy nothing at this volume.
- **Not RabbitMQ for the websocket fan-out**, even once multi-instance. It can
  do it (fanout exchange + one `exclusive`/`auto-delete` queue per instance),
  but see the traps below.

### Prerequisite: extract the publisher interface — done
`events/` now defines two interfaces: `Publisher` (`Publish(financeId, isSaving)
error`, held by `TransactionHandler`) and `Subscriber` (`Subscribe(ctx,
handler func(BroadCastMessage))`, consumed by `SharedFinanceWS.HandleBroadCast`).
`events.MemoryBroker` implements both over one `chan BroadCastMessage` (buffer
100): `Publish` builds the event with `BuildWebSocketEvent` and does a
non-blocking send (drops + logs when the buffer is full, so an ephemeral ping
never stalls the HTTP request); `Subscribe` ranges the channel until `ctx` is
cancelled. `main.go` constructs the broker, passes it to `HandleBroadCast` with
a cancellable `subCtx`, and threads it into `routes.TransactionRouter(api,
broker)`. Redis later is a new `events/redis_broker.go` implementing the same
two interfaces plus one line in `main.go`; nothing in `websockets/` or
`controllers/` changes. Traps:
- The producer side depends on `events.Publisher`, the ws goroutine on
  `events.Subscriber` — segregated so each caller sees only what it needs —
  but **one concrete struct implements both**, which is what makes "publisher
  and subscriber are the same object, receiving its own messages back" (see
  the Redis trap below) fall out for free.
- `TransactionRouter` taking an `events.Publisher` param is the one break from
  "every `*Router` takes only a `*gin.RouterGroup`". Test engines
  (`routes/authz_integration_test.go`) build their own `events.NewMemoryBroker()`
  and pass it in.
- `HandleBroadCast` is now `Subscribe(ctx, sfws.dispatch)` — `dispatch` already
  had the exact `func(events.BroadCastMessage)` signature the handler wants.
  `subCtx` is only torn down by `defer cancelSub()` on `main` return, i.e. after
  the graceful-shutdown `select`, not sequenced into it. Harmless for the
  in-memory broker (the channel has nothing to flush); a Redis broker that
  needs to drain in-flight work would want `cancelSub()` moved up next to
  `close(doneWS)`.

### What breaks at more than one instance
The complete list today:
1. `financeClients` (`websockets/websocket_handler.go`) plus
   `events.MemoryBroker`'s in-process channel — process-local connection
   registry and transport, so an event produced on instance A never reaches a
   client connected to B. The `events.Publisher`/`events.Subscriber` seam is
   already in place; this becomes swapping `MemoryBroker` for a Redis
   implementation of the same two interfaces. → Redis Pub/Sub.
2. `middlewares/rate_limiter.go` — per-process counters, so N instances give
   N× the effective limit. → Redis counters, plus Gin trusted-proxy config so
   `X-Forwarded-For` yields the real client IP behind a load balancer.

Auth is already multi-instance-ready: JWT is stateless, so there is no session
store to move.

**The trigger is not always scale.** The day a background worker needs to push
a websocket event (e.g. "invite email bounced, notify the finance"), the
cross-process broker is required regardless of instance count.

### Traps worth remembering
- **Instances never connect to each other.** Each connects only to Redis, and
  each is both publisher and subscriber on the same channel — *including
  receiving its own messages back*. That is deliberate: one code path for "an
  event arrived" instead of local-dispatch-here / broker-dispatch-there. Do
  not filter out own messages.
- **Redis Pub/Sub has zero buffering.** A message published while an
  instance's subscriber is reconnecting is gone for that instance, forever.
  Correct for these events, but it makes the subscriber's reconnect-with-
  backoff loop load-bearing: it must never exit. If "no missed events" is ever
  required, upgrade to Redis Streams (`XADD`/`XREADGROUP`) — same server, no
  new dependency.
- **Durability is a cost, not a free bonus.** RabbitMQ on the websocket path
  would replay a 400-message backlog to clients after a 30s network hiccup;
  suppressing that needs per-message TTL and `x-max-length`, i.e. paying for
  durability and configuring it back off. Worse, an orphaned queue (a
  `durable` misconfig, or a stale connection RabbitMQ hasn't reaped) accepts
  events forever with nobody draining it until the memory watermark **blocks
  publishers** — HTTP handlers then block on a queue for a server that no
  longer exists. Redis Pub/Sub cannot accumulate anything.
- **RabbitMQ cannot replace Redis here.** It is a broker, not a datastore — no
  `INCR`, no TTL'd counters — so the rate limiter needs Redis regardless.
  Using RabbitMQ for the fan-out never saves a dependency.
- **Ack after the work succeeds, never on receive.** Ack means "safe to
  delete"; acking early turns a crash mid-send into a lost email. Set a small
  `prefetch`, otherwise one consumer grabs the whole queue and adding
  consumers stops helping.
- **At-least-once means duplicates.** Give every event a UUID so consumers can
  dedupe, and decide explicitly whether to dedupe or tolerate the rare double
  send.
- **Rate limiter atomicity.** `INCR` then `EXPIRE` as two calls races: die in
  between and the key has no TTL, locking that IP out permanently. Use
  `SET key 0 EX 60 NX` first or a Lua script. Start with a fixed window — the
  window-boundary 2× burst is acceptable for login throttling. **Fail open**
  when Redis is down: taking down auth because the protection is unavailable
  is worse than the attack it prevents.
- **Publishing is a network call.** It must not fail the HTTP request for
  ephemeral events (log and return 201), and it needs a context/timeout.
  Committing the DB transaction and then publishing can disagree if the
  publish fails; for anything that must not be lost the fix is the
  **transactional outbox** (write the event to an `outbox` table inside the
  same DB transaction, a poller publishes and marks it sent). Not worth
  building for websocket pings.
- **Serialization forces a contract.** Once it crosses a process it is bytes:
  use JSON, add a version/type field now because rolling deploys mean old
  instances receive new instances' messages, and unknown types must be ignored
  rather than fatal. Never publish anything you wouldn't put in a log.
- Local dispatch does not change. `dispatch`, `financeClients`, `writePump`
  and the membership check stay exactly as they are — the broker only replaces
  the transport *between* processes. The slow-client drop
  (`websocket_handler.go`'s `default:` case) is still needed.

### Build order
1. ~~Extract the publisher interface. No infra.~~ Done — `events.Publisher` /
   `events.Subscriber` / `events.MemoryBroker`.
2. RabbitMQ + email worker as its own binary.
3. Stop there while single-instance.
4. Redis Pub/Sub + Redis rate limiter, when a second instance is actually
   needed or a worker needs to emit websocket events. Acceptance test: two
   `app` services in `docker-compose.yml`, a websocket client on each, create
   a transaction through one, assert the event arrives on both.

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
