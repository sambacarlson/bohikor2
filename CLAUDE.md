# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`AGENTS.md` is the canonical, actively-maintained project reference (stack, repo layout, auth/request
flow diagrams, code style, security rules, deploy checklist). Read it first — this file adds
architecture detail and command references that complement it rather than repeat it.

## Current state (2026, mid-pivot)

The project is a single-company salary-advance app **pivoting to multi-tenant** per `PLAN.md`
(Epics 6–9). Epic 6 (multi-tenant backend) and Epic 7 (payout reliability & float) are both done,
merged to `main`. Concretely this means:

- Every domain table now has `company_id`; a `companies` table and `platform_admins` (global
  super-admins) exist alongside the original `admins`/`users`.
- `admin/` has been renamed to `bohikor/` (Epic 8 task 1) — the directory move is done, but it is
  still the pre-pivot Next.js dashboard underneath. The route restructuring, unified employee/
  platform/admin screens, and de-shadcn work described in Epic 8 tasks 2–7 have not landed yet. Don't
  assume routes like `/{company}/admin` exist until those tasks land.
- `mobile/` is pre-pivot and will eventually be frozen (Epic 9), but is still the primary employee
  client for now.
- The migration set is greenfield and rewritten in place — there is no production data to preserve,
  so `backend/migrations/000001_schema.up/down.sql` is the *only* migration pair. New schema changes
  should extend or replace this file rather than accumulate numbered migrations mid-epic, until the
  baseline stabilizes.

Check `PLAN.md` for the target end-state and per-epic task breakdown before assuming a feature
described there already exists — it documents both shipped work and the plan.

## Commands

### Backend (`backend/`, Go 1.26)

```bash
make run               # go run cmd/server/main.go
make test              # go test -v -race -coverprofile=coverage.out ./...
make test-cover        # coverage restricted to business-logic packages, see below
make lint              # golangci-lint run
make generate          # cd db && sqlc generate — run after editing db/queries/*.sql
make migrate-up / migrate-down / migrate-create NAME=...
make docker-build
```

Run a single test: `go test ./internal/handler/... -run TestCreateRequest -v`

`make test-cover` computes coverage only over `internal/{handler,server,middleware,campay,service,
authjwt,email,authpassword}` — it deliberately excludes `db/sqlc` (generated), `cmd/*` (thin
entrypoints), `config/` (env loading), `repository/` (pgx pool wrapper) and `database/` (migration
runner). Use this target, not raw `go test -cover`, when checking whether coverage meets the bar.

Integration tests in `internal/server/integration_test.go` spin up Postgres via testcontainers and
run real migrations; they **skip automatically** (not fail) when Docker isn't available, so a green
`make test` locally without Docker is expected to under-report coverage of `server.New` wiring.

sqlc is configured in `backend/db/sqlc.yaml`: schema comes from `../migrations`, queries from
`db/queries/*.sql`, output goes to `db/sqlc` (committed, not gitignored — see AGENTS.md deploy
checklist for why). `uuid`→`google/uuid.UUID`, `numeric`→`shopspring/decimal.Decimal`,
`timestamptz`→`time.Time` (nullable → `sql.NullTime`).

### Bohikor (`bohikor/`, Next.js 16) and Mobile (`mobile/`, Expo 54)

Both use `npm run {dev,lint,typecheck,test}`; mobile additionally has `android`/`ios`/`web`. See
AGENTS.md for full build/run details — commands there are current.

## Backend architecture

Layering, thinnest to thickest:

- **`internal/server`** — composition root. `server.go` builds every dependency (db pool, migrations,
  JWT/bcrypt services, email client, Campay client, sqlc `Queries`) and wires routes in one function;
  there's no DI container. `routes.go` also defines two trivial handlers (`/me` for user/admin) inline
  rather than in `internal/handler`, since they're one-liners over the sqlc `Queries` interface.
- **`internal/handler`** — one file per resource (`auth.go`, `advance.go`, `platform.go`, etc.), each
  typically exposing a constructor (`NewAuthHandler`, `NewAdvanceHandler`, ...) that closes over a
  narrow interface (not the full `db.Queries` struct) so handlers are unit-testable with fakes.
  `handler.go` holds the shared `JSONError`/`JSONSuccess`/`JSONOK` response helpers used everywhere.
- **`internal/middleware`** — `JWTAuth` parses the bearer token and sets `subject_id`, `subject_type`,
  `claim_company_id` on the Gin context. Downstream, `RequireAdmin` / `RequireActiveUser` /
  `RequirePlatformAdmin` load the actual DB record, verify it, and **re-check that the row's
  `company_id` matches the JWT's `claim_company_id`** (the "isolation guarantee" — this is the load-
  bearing multi-tenancy check, not just role gating) before setting `company_id` in context and
  calling `c.Next()`. `RequireAdmin`/`RequireActiveUser` also reject if the company itself is
  suspended (`companyActive` helper in `role.go`).
- **`internal/service`** — cross-cutting business logic that doesn't belong to one HTTP resource
  (currently `invite.go`). Handlers depend on service interfaces, not concrete structs, mirroring the
  handler pattern.
- **`db/sqlc`** — generated; never hand-edit. Regenerate via `make generate` after touching
  `db/queries/*.sql` or the migration schema.

**Route groups in `routes.go`** follow one shape: `router.Group(prefix)` →
`.Use(authMiddleware)` → `.Use(middleware.Require*)` → handler registration. When adding an endpoint,
match the group whose guard already encodes the right role/tenancy check rather than gating inside
the handler. Public/unauthenticated routes (`/api/auth/login`, `/health`, the Campay webhook) live
outside any guarded group.

**Multi-tenancy contract:** every sqlc query touching tenant data takes `company_id` as a parameter
sourced from the JWT claim (via the middleware-set context value), never from the request body or URL.
`platform_admin` is the one subject type with no `company_id` (empty claim) and is gated by
`RequirePlatformAdmin` instead.

**Payout/webhook flow** (`advance.go`, `internal/campay`): `CreateRequest` and `RetryRequest` both
route through `processAdvanceRequest`, which checks the **isolation guarantee**'s tenancy scoping
plus eligibility (kill switch, request window, daily/monthly limits, company float), then debits
float and creates the request row in one DB transaction (`RealAdvanceStore.CreateAdvanceRequestWithDebit`,
which also takes a `SELECT ... FOR UPDATE` row lock on the company to keep the float check atomic
under concurrent requests) before calling Campay's Withdraw API. The Campay outcome is mapped by
the shared `disburseAndResolve` helper: `SUCCESSFUL`/`PENDING` → `success`/`pending`; a
Campay-declined transfer → `failed` (+ ledger reversal); a transport error/timeout (no response at
all) → `processing`, **never** `failed` — the whole point of `processing` is that we don't yet know
if money moved. Async confirmation arrives via `POST /v1/webhooks/campay` (JWT-signed, HS256),
routed through the same shared `service.TransitionRequest` helper (`internal/service/payout.go`)
that the webhook, the reconciler, and admin actions all call — it is the single place that mutates
`advance_requests.status`, posts the ledger reversal on a transition to `failed`, and makes
terminal statuses (`success`/`failed`) immutable once reached.

A background reconciler (`internal/reconciler/`, started in `server.New`, stopped on graceful
shutdown) polls `processing`/`pending` rows with a `campay_payout_ref` via
`campay.Client.GetTransactionStatus` on a 30s tick with backoff (30s/1m/2m/5m/15m), escalating to
`needs_admin_review` after 5 attempts. Rows with no ref (a pure timeout — nothing to poll) instead
get a 10-minute grace period before the same escalation, relying on the webhook's
`external_reference` fallback lookup to resolve them if Campay's confirmation arrives late.
Recovery paths: `POST /api/advance-requests/:id/retry` (employee, only on a terminally `failed`
request) creates a fresh row through the same eligibility+float+Campay flow; company admins get
`POST /api/admin/requests/:id/{reconcile,resolve,reissue}` (force an immediate poll; clear a
`needs_admin_review` flag with an audited note; or re-debit and re-disburse via Campay for a
`failed` request, blocked from double-reissue by a DB unique constraint on `reissued_from_id`).

## Git/PR workflow

Never delete remote branches.

## Frontend architecture

Both `bohikor/` and `mobile/` follow the same shape: `src/lib/api.ts` (axios instance with JWT
interceptor + refresh-token rotation), `src/lib/auth.ts` (token storage), `src/hooks/use-*.ts`
(TanStack Query hooks, one file per resource, mirroring the backend handler-per-resource split), and
a router-driven `app/` tree (Next.js App Router in `bohikor/`, Expo Router in `mobile/`) with route
groups for `(auth)` vs authenticated screens.
