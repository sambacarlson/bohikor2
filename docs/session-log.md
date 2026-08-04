# Session Log

## 2026-08-04 — Epic 8 backend gaps: company ledger view, platform request health, invite links (Complete)
- Found while scoping the rest of Epic 8 (frontend): three read/plumbing gaps in the API surface
  that the still-stubbed `bohikor/` screens will need. All additive, no schema changes.
- **`GET /api/admin/ledger`** (`internal/handler/ledger.go`) — company admin's own balance +
  paginated ledger history (`ListLedgerByCompany`/`GetCompanyBalance` already existed as sqlc
  queries but were only wired to platform-admin routes). Always scoped to the caller's own
  `company_id` from context, never a param.
- **`GET /api/platform/requests/needs-review`** and **`GET /api/platform/requests/health`**
  (`internal/handler/platform.go`) — the reconciler's `needs_admin_review` escalation path
  (Epic 7) previously surfaced only per-company via `/api/admin/requests`; a platform admin had no
  cross-tenant visibility into stuck payouts. New sqlc queries
  `ListRequestsNeedingReviewAcrossCompanies` and `ListCompanyRequestHealth` (counts of
  processing/pending/needs-review per company) back these; both are read-only.
- **Invitation emails now carry a real link.** `internal/email/email.go SendInvitation` previously
  sent plain text with no URL at all. Added `Config.FrontendBaseURL` (`FRONTEND_BASE_URL` env var,
  defaults to `http://localhost:3000`), threaded the company's slug through
  `InviteService.Invite` (new `AdminQuerier.GetCompanyByID` method) to build
  `{FRONTEND_BASE_URL}/{company-slug}/signup?email={encoded}`, matching PLAN.md's Epic 8 task 6.
- `go test -race ./...` and `golangci-lint run` both clean after the change.

## 2026-08-01 — Epic 8 task 1: rename admin/ → bohikor/ (Complete)
- Mechanical rename only: `git mv admin bohikor`, `package.json` name updated to `bohikor`, `npm install && npm run build` verified unchanged, `admin/` references in `AGENTS.md`/`CLAUDE.md` updated — no route/component changes (later Epic 8 tasks).

## 2026-08-01 — Epic 7: Payout reliability & float (backend) (Complete)

### What we did
- **`TransitionRequest`** (`internal/service/payout.go`) — the single place that mutates
  `advance_requests.status`: posts the `reversal` ledger entry on any transition to `failed`,
  emits `payout_success`/`payout_failed` events, and no-ops on an already-terminal row so the
  webhook, the reconciler, and admin actions can't double-post a reversal or race each other.
  `CreateRequest`, `RetryRequest`, the webhook handler, and the reconciler all route through it
  instead of hand-rolling status updates.
- **Ledger-gated payout** — `RealAdvanceStore.CreateAdvanceRequestWithDebit` creates the request
  row and posts a `payout_debit` ledger entry in one transaction, with a `SELECT ... FOR UPDATE`
  row lock on the company (`LockCompanyForFloatCheck`) so the float check is atomic under
  concurrent requests — re-checking the balance without the lock does not close the race under
  Postgres' default READ COMMITTED isolation. Campay outcomes map through a new shared
  `disburseAndResolve` helper: transport error/timeout → `processing` (never `failed` — we don't
  yet know if money moved); declined → `failed` + reversal; success/pending pass through.
- **`GetTransactionStatus`** (`internal/campay/client.go`) — polls `GET /transaction/{reference}/`.
  Confirmed against Campay's docs and official Python SDK that this endpoint accepts only
  Campay's own `reference`, never our `external_reference` — a real, separate bug in the existing
  webhook handler was found during this research (a webhook arriving for a `campay_payout_ref`-less
  row was silently dropped as "unknown reference") and fixed with an `external_reference` fallback
  lookup.
- **Reconciler** (`internal/reconciler/`) — background goroutine, 30s tick, started in
  `server.New` and stopped on graceful shutdown. Polls ref'd `processing`/`pending` rows on a
  backoff ladder (30s/1m/2m/5m/15m), escalating to `needs_admin_review` after 5 attempts; rows
  with no ref (nothing pollable) get a 10-minute grace period before the same escalation instead.
- **Recovery endpoints** — `POST /api/advance-requests/:id/retry` (employee, terminally-failed
  only, re-runs the full eligibility+float+Campay flow); `POST /api/admin/requests/:id/{reconcile,resolve,reissue}`
  (company admin — force a poll; clear `needs_admin_review` with an audited note; or re-debit and
  re-disburse via Campay for a `failed` request, guarded against double-reissue by a DB unique
  constraint on the new `reissued_from_id` column).
- **`POST /api/platform/companies/:id/ledger/adjustment`** — platform-admin manual ledger
  correction, mirroring the existing `ledger/topup` route.
- **Schema** — `advance_requests.reissued_from_id UUID UNIQUE REFERENCES advance_requests(id)`,
  additive to the single `000001` baseline per repo convention.

### Bugs found and fixed along the way
- **Money-correctness**: the original post-transfer-DB-write-failure fallback forced a request to
  `failed` even when Campay had already confirmed success, which would have posted an erroneous
  ledger reversal for a payout that actually went through — fixed to target `processing` instead.
- **Cross-tenant IDOR**: the admin reconcile/resolve/reissue endpoints didn't check that the
  target row belonged to the acting admin's own company, letting an admin act on — and reissue
  (debit) — another company's request. Fixed with a `companyIDFromContext` load-then-check
  pattern, 404 on mismatch.
- **Reconciler query gaps** (two rounds): `ListReconcilableAdvanceRequests` initially missed
  `processing` rows with no ref and a NULL `next_retry_at`, then a second sibling gap for ref'd
  rows in the same state — both closed with regression tests.
- **Final whole-branch review** caught one more Critical issue after all ten tasks were "done":
  the admin **reissue** endpoint debited float and created a new row but never actually called
  Campay, permanently stranding the row at `initiated` and blocking the employee from any new
  request. Fixed by extracting the Campay-call-and-outcome-mapping logic into a shared
  `disburseAndResolve` helper and reusing it in reissue. Also fixed in the same pass: the float
  check's TOCTOU race (needed the row lock described above — a naive re-check inside the debit
  transaction does not close it); `TransitionRequest`'s no-op guard only blocked re-delivery of
  the *same* terminal status, not a flip between two different terminal statuses; two admin
  endpoints (`resolve` on a non-flagged row, double-reissue) returned a generic 500 instead of a
  clean 409. A scoped re-review — independently re-running the tests, not just reading the fix
  report — confirmed all of these closed, and caught one more Minor issue in the fix itself
  (`disburseAndResolve` reporting the wrong HTTP status in two rare DB-write-failure sub-cases),
  fixed immediately.

### Tests & checks
- New: a two-goroutine concurrency test proving the float-check race is actually closed (two
  concurrent debits against a shared balance — exactly one succeeds, final balance correct);
  Campay outcome coverage for reissue (success/declined/transport-error); reconciler backoff and
  no-ref grace-period behavior; terminal-status immutability.
- `make lint` 0 issues; `make test` (`-race`, real Postgres via testcontainers, all packages)
  green; `make test-cover` 88.8% total (business-logic packages) — down slightly from ~92%/89.2%
  earlier baselines, expected given the reconciler package and new admin/platform endpoints added
  alongside their tests.

### Process notes
- Implemented via Subagent-Driven Development (10-task plan, fresh implementer + task review per
  task, in an isolated worktree), followed by a final whole-branch review and one bundled fix
  wave for everything it found.
- Merged (squash) into `main` via PR #3.

## 2026-07-29 — Epic 6: Multi-tenant foundation (backend) (Complete)

### What we did
- **Migrations rewritten to a clean multi-tenant baseline** (`000001_schema.up/down.sql`) — deleted stale `000002`–`000007` (authorized greenfield). Added `platform_admins`, `companies`, `company_ledger`; every domain table gains `company_id NOT NULL` (nullable only on `events`); new enums `company_status` and `request_status` value `processing`; `settings` PK becomes composite `(company_id, key)`; `refresh_tokens.subject_type` allows `platform_admin`; advance-request resilience columns (`attempt_count`, `last_reconciled_at`, `next_retry_at`, `needs_admin_review`) baked in now for Epic 7. Validated up→down→up against real Postgres 18.4.
- **sqlc queries** — new `companies.sql`, `platform_admins.sql`, `company_ledger.sql` (CreateLedgerEntry, GetCompanyBalance=SUM, list); threaded `company_id` through users/admins/invitations/advance_requests/phone_verifications/events; `settings.sql` composite key + `SeedDefaultSettings`. Pre-auth queries (GetUserByEmail, GetAdminByEmail, GetActiveInvitationByEmail, both OTP tables) stay email-only.
- **JWT + middleware** — access token gains a `company_id` claim (empty for platform admins); `generateTokenPair` takes company id; refresh **re-derives** company from the subject. `RequireAdmin`/`RequireActiveUser` assert `record.company_id == claim` (403 "company mismatch") and reject suspended companies; new `RequirePlatformAdmin`.
- **Platform handler** (`platform.go`) under `/api/platform/*` — create company (tx: company row + seeded settings via `RealPlatformStore.CreateCompanyWithSettings`), create first admin, ledger `topup`, list/detail with computed balance, suspend/activate.
- **Auth** — `Login` resolves company from email, blocks suspended companies, returns `company_slug`; new `POST /api/auth/platform/login`. `CreatePin` sources the new user's `company_id` from their invitation.
- **Company scoping** threaded through invitations/users/settings/events/advance/phone handlers; every `CreateEvent` passes `company_id`.
- **CLI** — `cmd/create-platform-admin` (bootstrap super-admin); `cmd/create-admin` now takes `--company` slug.
- **Docs** — rewrote `docs/schema.md` as the multi-tenant source of truth.

### Tests & checks
- New: middleware company-mismatch/suspended-company/platform-admin gating, platform provisioning + topup + slug validation, `Login` company resolution + suspended block, migration round-trip (opt-in via `MIGRATION_TEST_DB_URL`).
- Updated all existing tests for the new signatures. `go test ./...` green, `gofmt` clean, `golangci-lint` (v2) 0 issues.

### Notes / follow-ups
- `AdminLogin` does not yet return `company_slug` or gate suspended companies at login (middleware catches the latter on the next call) — address in Epic 8 routing.
- Local `golangci-lint` was v1 vs the repo's v2 config; installed v2.1.6 to run the real lint.

## 2026-06-05 — Epic 4: Settings engine, request controls, OTP rate limiting (Complete)

### What we did
- **Migration 000006** — `settings` table (JSONB key-value), `email_otp_failures` table, dropped old `amount_xaf` CHECK (replaced with 100-25000 range), seeded defaults
- **sqlc queries** — `settings.sql` (ListSettings, GetSetting, UpsertSetting), `email_otp_failures.sql` (GetEmailOTPFailure, UpsertEmailOTPFailure, ResetEmailOTPFailures), extended `advance_requests.sql` (CountAdvanceRequestsByUserToday, CountSuccessfulAdvanceRequestsByUserThisMonth)
- **Backend: Settings handler** (`settings.go`) — `GET/PUT /api/admin/settings` with value coercion (admin UI sends strings, handler converts to proper JSON primitives for JSONB storage)
- **Backend: Advance handler rewrite** — kill switch guard, request window (Africa/Douala TZ, configurable start/end day, 0 = last day of month), daily/monthly throttling, dynamic amount from settings, `GET /advance-requests/eligibility` endpoint returning all pre-conditions + reasons, `ListUserRequests`, `HandleListAdminRequests`, webhook handler, terms acceptance
- **Backend: Auth handler OTP rate limiting** — `checkEmailOTPBlocked` (permanent/temp block check), `recordOTPFailure` (3 same-day → block until midnight UTC, 6 total → permanent lock + user locked), `resetOTPFailures` (on successful OTP), integrated into `SendEmailOTP`, `VerifyEmailOTP`, `ForgotPin`
- **Backend: Users handler** — `HandleSuspendUser`, `HandleActivateUser`, `HandleUnlockUser` now also clears OTP failure counter
- **Backend: Server wiring** — settings routes, suspend/activate routes, eligibility route, timezone loading from config
- **Admin: Settings page** (`app/(main)/settings/page.tsx`) — card-based form for all 4 sections (advance amount, kill switch, request window, rate limits) with edit-toggle UX (inputs disabled until Edit clicked, Save re-disables)
- **Admin: Hooks** (`use-settings.ts`) — query + mutation hooks with correct payload format (`{[key]: value}`)
- **Admin: Sidebar** — added Settings nav item
- **Mobile: Eligibility hook** (`use-eligibility.ts`) — 30s polling query exposing `data`/`isLoading`/`refetch`/`isRefetching`
- **Mobile: Home screen** — dynamic advance amount from API, eligibility status card (eligible badge, remaining requests, window info, kill switch banner, reasons list, refresh button)
- **Mobile: Types** — `EligibilityResponse`, `RequestWindow` added
- **Fixed bugs during testing** — `admin_id` type mismatch (middleware sets string, handler expected uuid.UUID), settings payload format mismatch (frontend sent `{key, value}`, backend expected `{[key]: value}`), setting key name mismatches between frontend and backend (e.g. `advance_amount` vs `advance_amount_xaf`), JSONB value type coercion (admin UI sends strings, loadSettings needs native types)
- **render.yaml deleted** — stale, unused after move to Render dashboard config

### Key decisions
- Settings as JSONB key-value (not typed columns) — single migration for all future settings, extensible without schema changes
- Value coercion on write (not on read) — `coerceValue()` normalizes strings to numbers/bools before JSONB stores them; `parseJSONFloat`/`parseJSONBool` in `loadSettings` handle both formats as belt-and-suspenders
- OTP blocking: 3 consecutive failures same day → temp block until end of day UTC; 6 total consecutive → permanent lock + `LOCKUSER`; counter only resets on successful verification
- Request window: `end_day=0` means "last day of month", evaluated dynamically per request
- Daily/monthly limits: 0 = unlimited (skips check entirely)
- Advance amount bounds: 100-25,000 XAF (CHECK constraint on `advance_requests.amount_xaf`)
- Eligibility endpoint is read-only (no side effects) — mobile polls every 30s + manual refresh

### Verification
- Backend: `go test ./...` (13 suites), `golangci-lint run` (0 issues)
- Admin: `npm run lint` + `npm run typecheck` + `npm test` (28 tests)
- Mobile: `npm run lint` + `npm run typecheck` + `npm test` (26 tests)
- Commit: `a7c0e70`

## 2026-06-05 — Phone verification to Collect API, payout hardening, USSD flow (In Progress → Completed)

### What we did
- **Campay client** (`client.go`): added `CollectRequest`, `CollectResponse` types, `InitiateCollection()` calling `POST /collect/`, shared `campayTransferer` interface extended with `InitiateCollection`, comprehensive unit tests for collection flow
- **Phone handler** (`phone.go`): switched from `InitiateTransfer` to `InitiateCollection`, returns `ussd_code` in POST + GET responses, fixed `amount_xaf` to use `h.verifAmt` instead of zero, added `phone_verification_initiated` event, added fallback-to-failed on post-collect DB error
- **Advance handler** (`advance.go`): added user status check to `CreateRequest` (forbidden if not active), added fallback-to-failed on post-transfer DB error (money already sent but DB update fails → mark failed with reason), webhook dedup (skip event when status unchanged), SetPhoneVerified failure returns 500 to trigger Campay retry
- **Migration 000007**: added `ussd_code TEXT` column to `phone_verifications` table
- **sqlc**: updated query to include `ussd_code`, regenerated via `go generate ./db/...`
- **Mobile phone screen** (`settings/phone.tsx`): reads `ussd_code` from GET endpoint, displays it with dialing instructions, added `canRetry` logic (failed always retryable, initiated/pending only after 1 minute cooldown)
- **Mobile types** (`index.ts`): `PhoneVerificationStatus.verification.ussd_code?: string`
- **Mobile auth hook** (`use-auth.ts`): minor fix

### Key decisions
- Phone verification uses Campay Collect API (USSD debit) instead of mini-withdrawal — user dials USSD code from phone dialer, enters PIN, webhook confirms
- Post-Campay DB failures mark the record as `failed` (with campay_ref saved) and return 500 — no permanently stuck records
- Webhooks return 500 when critical side-effects fail (e.g. `SetPhoneVerified`) — Campay retry mechanism engages
- USSD code stored in DB column (not ephemeral) — survives app restart, returned from both POST and GET endpoints
- Mobile retry enabled after 1 minute for initiated/pending — gives user time to dial USSD before offering retry
- Both collect and withdraw flows hardened: post-transfer/post-collect DB error → fallback to failed + log

### Deferred
- Kill switch inconsistency: eligibility endpoint correctly reflects `kill_switch_active: false`, but `CreateRequest` may reject with kill switch error. Both use identical `loadSettings()` → `checkKillSwitch()` path. Likely request window blocking (start_day=15, today before 15th) but user reports kill switch error. Needs investigation when resolved.

### Verification
- Backend: `go test ./...`  — passes
- Backend: `golangci-lint run` — passes
- Mobile: `npm run lint` + `npm run typecheck` — passes
- Committed in this session alongside PLAN.md and docs updates

## 2026-06-02 — Epic 2.5: Firebase removal, own auth migration (Complete)

### What we did
- **Backend: authjwt package** — `internal/authjwt/service.go` (TokenService interface + HS256 impl), `token_util.go`, `service_test.go`
- **Backend: authpassword package** — `internal/authpassword/hasher.go` (Hasher interface), `bcrypt.go`, `hasher_test.go`
- **Backend: sms package** — `internal/sms/sender.go` (Sender interface); `sms/africastalking/client.go` (production SMS via Africa's Talking); `sms/discord/client.go` (dev SMS via Discord webhook with embed)
- **Backend: Config** — `SMS_PROVIDER` env var (`discord`/`africastalking`, default `discord`), `DISCORD_WEBHOOK_URL`, `DISCORD_BOT_USERNAME`
- **Backend: server.go** — switch on `SMS_PROVIDER` to wire discord or africastalking Sender
- **Backend: Migration 000004** — drops `firebase_uid` from `users` and `admins`, adds `password_hash` to `admins`, creates `phone_otps` and `refresh_tokens` tables
- **Backend: sqlc queries** — rewrote users.sql, admins.sql; new phone_otps.sql, refresh_tokens.sql
- **Backend: Auth handler** — SendPhoneOTP, VerifyPhoneOTP, AdminLogin, RefreshToken, Logout, CheckInvitation, SendEmailOTP, VerifyEmailOTP
- **Backend: JWTAuth middleware** — RequireAdmin/RequireActiveUser using subject_id/subject_type from JWT claims
- **Backend: Routes** — handleUserMe/handleAdminMe using UUID from JWT `sub` claim
- **Backend: CLI tool** — `cmd/create-admin/main.go` accepts --email and --password (replaces seed data)
- **Mobile: lib/auth.ts** — token storage via expo-secure-store (~15.0.8 for SDK 54)
- **Mobile: lib/api.ts** — axios interceptor attaches Bearer token, 401 response interceptor does refresh with rotation
- **Mobile: providers/auth-provider.tsx** — token-based auth context exposing {user, loading, signOut, refreshUser}
- **Mobile: All auth screens** — login.tsx, verify-phone.tsx, home.tsx, terms.tsx use useAuth() instead of Firebase
- **Mobile: Firebase fully removed** — uninstalled @react-native-firebase/*, firebase; deleted google-services.json, GoogleService-Info.plist, native plugin; removed expo plugin from app.json
- **Admin: Firebase fully removed** — deleted lib/firebase.ts, uninstalled firebase package, removed Firebase env vars
- **Admin: lib/auth.ts** — localStorage token storage
- **Admin: lib/api.ts** — axios interceptor with Bearer token + refresh rotation; redirects to /login on 401
- **Admin: auth-provider.tsx** — token-based auth context; tries /api/admin/me then /api/users/me; exposes {user, admin, subjectType, loading, signOut, refreshSubject}
- **Admin: login/admin/page.tsx** — email/password via POST /api/auth/admin/login
- **Admin: login/page.tsx** — phone OTP via POST /api/auth/send-phone-otp + verify-phone-otp
- **Admin: auth-guard.tsx, sidebar.tsx, forbidden.tsx, client layout** — all use useAuth() instead of Firebase
- **Admin: All 63 tests passing** — rewritten to mock @/lib/api and @/lib/auth instead of firebase
- **Mobile: All 26 tests passing** — rewritten home.test.tsx and terms.test.tsx to mock useAuth from auth-provider
- **All lint + typecheck clean** — backend, admin, mobile

### Key decisions
- Dropped `firebase_uid` entirely (no prod data, wipe OK) rather than making nullable
- No admin seed data — `cmd/create-admin` CLI tool only
- Discord webhook for dev OTP delivery, Africa's Talking for production, controlled by `SMS_PROVIDER` env var
- Token strategy: HS256 JWT access tokens (15min) + opaque refresh tokens (30 days) with rotation
- Backend identifies users by UUID `sub` claim in JWT, not Firebase UID
- All auth concerns behind Go interfaces (TokenService, Hasher, Sender) for future microservice extraction
- Phone OTPs stored as plaintext `code` (not hashed) — short-lived, 15min expiry, deleted on verification
- Refresh tokens use polymorphic `subject_id`/`subject_type` pattern (supports both users and admins)
- Mobile uses `useEffect` for auth-based redirects (not render-time `router.replace()`) to avoid React setState-during-render errors
- `expo-secure-store` must be ~15.0.8 for SDK 54 — version 56.x causes native module crash

### Key lessons
- Always run `npx expo install --fix` then `npx expo prebuild --clean` when changing native modules
- React strict linter rejects `router.replace()` during render — always use `useEffect` for navigation based on auth state
- Admin auth-provider needs `useEffect` for initial fetch with eslint-disable for set-state-in-effect (it's initialization, not a side effect driven by deps)
- `golangci-lint` catches `w.Write` unchecked error returns and misspellings — fix early

## 2026-05-27 — Client web app, role-based access control, audit & fixes

### What we did
- Added a **client-facing web app** into the existing Next.js admin dashboard — no new project
- **Login page**: phone number → Firebase OTP (invisible Recaptcha) → verify → `/client`
- **Admin login page**: moved to `/login/admin` with guard preventing double-login
- **Client pages**: Home (request advance + user info + terms banner), History (transaction list with auto-poll every 10s), Terms (acceptance flow)
- **Hooks**: `useUser(enabled)` — race-safe Firebase-ready gating; `useAdmin(enabled)` — same pattern; `useAdvanceRequests` with 5s staleTime; `useCreateAdvanceRequest`; `useAcceptTerms` with cache invalidation
- **Access control**: `AuthGuard` upgraded to verify admin role via `GET /api/admin/me` (403 for non-admins); `ClientLayout` detects admin access via 404 from `RequireActiveUser` middleware (403 for admins on client routes)
- **API layer**: `ApiError` type + `getApiErrorMessage()` helper, replacing fragile inline type assertions
- **UI components**: Radix `Checkbox` (was missing), `ForbiddenPage` (403 with sign-out + navigate)
- **Fixes during review**: formatPhone auto-prepends `+237` Cameroon country code; OTP error handling distinguishes invalid-code/expired/too-many-attempts; account-not-found state on login page when Firebase auth succeeds but backend user doesn't exist; client layout separates server error from account-not-found; terms page uses `router.push` instead of `router.back`; defensive date formatting (`isNaN` + null check); `useCallback` missing deps fix
- **Jest**: kept `jest` as global (project convention), added `jest.d.ts` type declarations for IDE support
- **Tests**: 8 suites, 65 tests all pass; lint 0/0; typecheck clean; backend tests all pass; build produces all 11 routes

### Key decisions
- Phone + OTP on same page (step toggle) to keep `confirmationResult` in memory — avoids losing it across navigation
- `useUser(enabled)` / `useAdmin(enabled)` pattern: hooks don't fire until Firebase auth resolves, preventing 401 race conditions on page load
- `RecaptchaVerifier` with `size: "invisible"` — no user-facing captcha; cleaned up on unmount via `verifierRef.current.clear()`
- Backend role middleware already returns correct codes: `RequireAdmin` → 403, `RequireActiveUser` → 404 — frontend just translates those into ForbiddenPage
- Tests use `jest` as global (not imported from `@jest/globals`) — importing breaks `babel-jest` hoisting of `jest.mock()`, causing `@firebase/auth` to load in JSDOM where `fetch` is undefined

## 2026-05-26 — Docker deploy fixes, EAS build, secrets audit

### What we did
- Fixed Render Docker deploy: set `dockerContext: ./backend` in `render.yaml`
- Fixed Firebase credentials temp file lifecycle in `backend/internal/firebaseapp/app.go`
- Replaced custom migration runner with `golang-migrate` in `backend/internal/database/migrate.go`
- Fixed `db/sqlc/` being gitignored (committed generated code so Docker build can find it)
- Fixed `migrations/` missing from Docker runtime stage (added `COPY --from=builder /app/migrations ./migrations/`)
- Ran EAS local Android build successfully (`build-1779550117750.aab`)
- Created Google Play Service Account and linked to EAS credentials
- Installed `golang-migrate/v4`, ran lint + tests
- Added pre-push checklist to `AGENTS.md`

### Key lessons
- Generated code (`db/sqlc/`) must be tracked in git — `.gitignore` shouldn't exclude build-time dependencies
- Multi-stage Docker builds need explicit `COPY` for runtime-accessed directories (migrations)
- Always run `docker build` locally before pushing Docker changes to Render
- Firebase Web API keys (`AIza...`) are public by design — GitHub secret scanning false positive
