# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`AGENTS.md` is the canonical, actively-maintained project reference (stack, repo layout, auth/request
flow diagrams, code style, security rules, deploy checklist). Read it first — this file adds
architecture detail and command references that complement it rather than repeat it.

## Current state (2026, mid-pivot)

The project is a single-company salary-advance app **pivoting to multi-tenant** per `PLAN.md`
(Epics 6–9). Epic 6 (multi-tenant backend) is done on `epic-6-multi-tenant-foundation`. Concretely
this means:

- Every domain table now has `company_id`; a `companies` table and `platform_admins` (global
  super-admins) exist alongside the original `admins`/`users`.
- `admin/` is still the pre-pivot Next.js dashboard — it has **not** yet been renamed/rebuilt into
  the unified `bohikor/` app described in Epic 8. Don't assume routes like `/{company}/admin` exist
  until that epic lands.
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

### Admin (`admin/`, Next.js 16) and Mobile (`mobile/`, Expo 54)

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

**Payout/webhook flow** (`advance.go`, `internal/campay`): synchronous call to Campay's Withdraw API
at request time, async confirmation via `POST /v1/webhooks/campay` (JWT-signed, HS256). Epic 7 (not
yet implemented) will add a reconciler and a `processing` state so a lost webhook or timeout no longer
strands a request — see `PLAN.md`'s state-machine diagram before touching request-status transitions.

## Frontend architecture

Both `admin/` and `mobile/` follow the same shape: `src/lib/api.ts` (axios instance with JWT
interceptor + refresh-token rotation), `src/lib/auth.ts` (token storage), `src/hooks/use-*.ts`
(TanStack Query hooks, one file per resource, mirroring the backend handler-per-resource split), and
a router-driven `app/` tree (Next.js App Router in `admin/`, Expo Router in `mobile/`) with route
groups for `(auth)` vs authenticated screens.
