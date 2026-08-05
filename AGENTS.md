# AGENTS.md — Bohikor2

Multi-tenant salary-advance platform. See `docs/brief.md` for business rules and `docs/schema.md`
for the data contract.

## Stack

- **Backend:** Go 1.26, Gin, sqlc, pgx/v5, golang-migrate, Resend
- **Bohikor:** Next.js 16, Tailwind v4, TanStack Query
- **Mobile:** Expo SDK 54, React Native 0.81, NativeWind, TanStack Query, expo-secure-store — **frozen**, see `mobile/README.md`
- **Database:** PostgreSQL 18.4 (Neon/Supabase)
- **Payments:** Campay Withdraw API (`POST /withdraw/`, sandbox: `https://demo.campay.net/api`)
- **Testing:** Go `testing`, Jest + RTL (bohikor), Jest + RNTL (mobile)

## Repo Structure

```
bohikor2/
├── docs/              # brief.md, schema.md, session-log.md
├── PLAN.md
├── AGENTS.md
├── backend/           # Go API (Gin + sqlc + golang-migrate)
│   ├── cmd/server/main.go
│   ├── internal/      # handlers, services, middleware, config, campay, reconciler
│   ├── db/queries/    # sqlc query definitions
│   ├── db/sqlc/       # generated Go code (do not edit)
│   ├── migrations/    # numbered .up.sql / .down.sql files
│   └── go.mod
├── bohikor/           # Next.js multi-tenant web app — employee `/{company}`,
│   └── src/           # company admin `/{company}/admin`, platform admin `/platform`
└── mobile/            # Expo app — frozen (see mobile/README.md), no longer the
    ├── app/           # primary employee client; kept green, no new features
    └── src/
```

## Build & Run

### Backend

```bash
cd backend && go mod download
go generate ./db/...          # regenerate sqlc after query changes
go run cmd/server/main.go     # dev server
go test ./...                  # tests
golangci-lint run              # lint
```

### Bohikor

```bash
cd bohikor && npm install
npm run dev       # dev server
npm run lint      # ESLint
npm run typecheck # tsc --noEmit
npm run test      # Jest
```

### Mobile

```bash
cd mobile && npm install
npx expo run:android   # or npx expo run:ios (requires prebuilt native dirs)
npm run lint           # ESLint
npm run typecheck      # tsc --noEmit
npm run test           # Jest + RNTL
```

Mobile requires a dev client build (`npx expo run:android/ios`). No Firebase native modules needed.

## Auth Flows (Epic 1 — Complete, Epic 2.5 — Complete, Epic 3 — PIN Auth Overhaul)

### Company Admin / Platform Admin
1. Sign in with email/password → backend bcrypt verification → JWT tokens
   (`/{company}/admin/login` for a company admin, `/platform/login` for a platform admin)
2. Company admin: users, invite, requests (+ reconcile/resolve/reissue), settings, balance/ledger,
   events — all scoped to their own company
3. Platform admin: companies (create + first admin + top-up balance + suspend/activate),
   cross-company request health / needs-review queue

### Mobile — Login (Returning User)
1. Enter email + 5-digit PIN → `POST /api/auth/login` → home

### Mobile — Signup (New User)
1. Enter invited email → `GET /api/auth/check-invite` → blocked if no invitation
2. `POST /api/auth/send-email-otp` → 6-digit code via Resend
3. Enter code → `POST /api/auth/verify-email-otp` (purpose=signup) → invitation marked accepted
4. `POST /api/auth/create-pin` → set 5-digit PIN → home

### Mobile — Forgot PIN
1. Enter email → `POST /api/auth/forgot-pin`
2. `POST /api/auth/verify-email-otp` (purpose=pin_reset)
3. `PUT /api/users/me/pin/reset` → set new PIN → home

### Phone Verification (Settings)
1. User adds phone number in settings → `POST /api/users/phone`
2. Backend calls Campay mini-withdrawal (1 XAF) → `POST /withdraw/`
3. Campay webhook confirms → phone marked verified
4. No SMS dependency — phone verification uses Campay payout flow

### PIN Rate Limiting
- 3 failed attempts per hour → 1hr cooldown → 3 more attempts → account locked
- Locked accounts: `status = 'locked'`, `locked_until` set
- Admin unlocks: `PUT /api/admin/users/:id/unlock`

**Edge cases:** User already exists → route to login. Invite accepted but PIN not created → route to create PIN. Suspended user → blocked. Locked user → blocked until admin unlocks.

## Request Flow (Epic 2, hardened in Epic 7 — Complete)

1. User taps "Request Advance" → confirmation modal
2. Backend checks: user active, phone verified, terms accepted, no in-flight request, kill switch,
   request window, daily/monthly limits, **company float ≥ advance amount**
3. `POST /api/advance-requests` → in one DB transaction (row-locked on the company to keep the
   float check atomic under concurrent requests), creates the request (`initiated`) and posts a
   `payout_debit` ledger entry reserving the float; then calls Campay's Withdraw API (`POST /withdraw/`)
4. Campay's response maps to a status: `SUCCESSFUL`/`PENDING` → `success`/`pending`; declined →
   `failed` (+ ledger reversal); timeout/transport error (no response at all) → `processing` —
   **never** `failed` for an unconfirmed outcome
5. Campay's async confirmation arrives via `POST /v1/webhooks/campay` (JWT-signed); a background
   reconciler also polls `processing`/`pending` rows on a backoff schedule, so a lost webhook or
   timeout doesn't strand a request
6. User sees status in transaction history; a terminally `failed` request can be retried
   (`POST /api/advance-requests/:id/retry`) or, by a company admin, reconciled/resolved/reissued
   (`POST /api/admin/requests/:id/{reconcile,resolve,reissue}`)

**Terms:** Must be accepted before requesting. Separate screen from auth. Stored on `users.is_terms_accepted`.

## Code Style

### Go
- `gofmt` formatting, `golangci-lint run` must pass
- All `UPDATE` queries must set `updated_at = NOW()`
- Use `slog` with structured fields, no string interpolation for errors
- Migrations: numbered sequentially, no `IF NOT EXISTS`/`IF EXISTS`. Never modify an existing
  migration file — always add a new one (`make migrate-create NAME=...`) for schema changes.
- Config via environment variables (`caarlos0/env`)

### TypeScript
- Strict mode in all `tsconfig.json`
- Prettier + ESLint
- TanStack React Query for all server state
- Axios interceptors attach JWT access tokens

### Database
- Schema source of truth: `docs/schema.md`
- `snake_case` tables/columns, `idx_<table>_<desc>` for indexes
- All timestamps `TIMESTAMPTZ`, IDs as UUIDs

## Testing

Every PR must pass lint + typecheck + tests for the changed workspace(s).

## Security

- Never commit secrets or API keys
- Use environment variables for all config (`.env.example` files with dummy values)
- Admin endpoints require JWT auth via `RequireAdmin` middleware
- Campay webhooks must be JWT-verified (HS256, signature in body) before processing
- PINs are hashed with bcrypt before storage — never stored or logged in plaintext

## Deploy

`backend/` deploys as a Docker container built from `backend/Dockerfile`. `bohikor/` is built
directly from source by the deployment platform (no Dockerfile) — the checklist below applies to
`backend/` only.

### Pre-push checklist

Before pushing a backend change that touches Docker, build config, or migrations:

1. **`docker build` locally** — run from repo root:
   ```bash
   docker build -f backend/Dockerfile -t bohikor2-test backend/
   ```
   Catches missing files, bad paths, or broken builds before the deploy platform does.

2. **Verify tracked files** — check that all needed files are tracked:
   ```bash
   git ls-files backend/db/sqlc/ backend/migrations/
   ```
   Generated code (`db/sqlc/`) must be committed, not gitignored. The `AGENTS.md` comment "do not edit" is convention — the files still need to exist in the build image.

3. **Check runtime assets in the image** — any directory the binary reads at runtime (migrations, templates, static files) must be `COPY`ed into the Docker runtime stage. Multi-stage builds don't carry files forward automatically.

4. **Audit `.gitignore`** — confirm it only excludes truly ephemeral files (`.env`, `node_modules`, binaries). Never exclude build-time dependencies (generated code, SQL files, config templates).

## Rules

- **NEVER auto-commit changes.** Always wait for explicit user instruction to commit.
- When in doubt, ask before implementing.
- Run lint + typecheck + tests before suggesting a change is complete.
