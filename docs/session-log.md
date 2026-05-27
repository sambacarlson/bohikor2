# Session Log

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
