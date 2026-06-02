# Session Log

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
