# AGENTS.md — Bohikor2

Salary advance pilot app. See `docs/brief.md` for business rules and `docs/schema.md` for the data contract.

## Stack

- **Backend:** Go 1.26, Gin, sqlc, pgx/v5, golang-migrate, Resend
- **Admin:** Next.js 16, shadcn/ui, Tailwind v4, TanStack Query
- **Mobile:** Expo SDK 54, React Native 0.81, NativeWind, TanStack Query, expo-secure-store
- **Database:** PostgreSQL 18.4 (Neon/Supabase)
- **Payments:** Campay Withdraw API (`POST /withdraw/`, sandbox: `https://demo.campay.net/api`)
- **Testing:** Go `testing`, Jest + RTL (admin), Jest + RNTL (mobile)

## Repo Structure

```
bohikor2/
├── docs/              # brief.md, schema.md
├── PLAN.md
├── AGENTS.md
├── backend/           # Go API (Gin + sqlc + golang-migrate)
│   ├── cmd/server/main.go
│   ├── internal/      # handlers, services, middleware, config, campay
│   ├── db/queries/    # sqlc query definitions
│   ├── db/sqlc/       # generated Go code (do not edit)
│   ├── migrations/    # numbered .up.sql / .down.sql files
│   └── go.mod
├── admin/             # Next.js dashboard (invite + users + requests)
│   └── src/
└── mobile/            # Expo app (email + PIN login + signup + advance requests)
    ├── app/           # Expo Router routes
    └── src/           # hooks, providers, types, lib
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

### Admin

```bash
cd admin && npm install
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

### Admin Dashboard
1. Sign in with email/password → backend bcrypt verification → JWT tokens
2. Dashboard: **Invite** (send invitation emails) and **Users** (list + refresh + unlock)

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

## Request Flow (Epic 2 — Complete)

1. User taps "Request Advance" → confirmation modal
2. Backend checks: user active, phone verified, terms accepted, no in-flight request
3. `POST /api/advance-requests` → creates request, calls Campay Withdraw API (`POST /withdraw/`)
4. Campay processes → sends webhook to `POST /api/webhooks/campay`
5. Backend verifies JWT signature, updates request status
6. User sees status in transaction history

**Terms:** Must be accepted before requesting. Separate screen from auth. Stored on `users.is_terms_accepted`.

## Code Style

### Go
- `gofmt` formatting, `golangci-lint run` must pass
- All `UPDATE` queries must set `updated_at = NOW()`
- Use `slog` with structured fields, no string interpolation for errors
- Migrations: numbered sequentially, no `IF NOT EXISTS`/`IF EXISTS`
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
- Migrations run in CI pre-deploy, never on app boot

## Testing

Every PR must pass lint + typecheck + tests for the changed workspace(s).

## Security

- Never commit secrets or API keys
- Use environment variables for all config (`.env.example` files with dummy values)
- Admin endpoints require JWT auth via `RequireAdmin` middleware
- Campay webhooks must be JWT-verified (HS256, signature in body) before processing
- PINs are hashed with bcrypt before storage — never stored or logged in plaintext

## Deploy

### Pre-push checklist

Before pushing a backend change that touches Docker, build config, or migrations:

1. **`docker build` locally** — run from repo root:
   ```bash
   docker build -f backend/Dockerfile -t bohikor2-test backend/
   ```
   Catches missing files, bad paths, or broken builds before Render fails.

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
