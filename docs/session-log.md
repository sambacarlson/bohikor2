# Session Log

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
