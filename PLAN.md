# PLAN.md — Bohikor2

## Epic 1: Authentication (Complete)

- Backend: Firebase Admin SDK, Resend email OTP, auth middleware, role middleware, invitation CRUD, user creation, email/phone verification endpoints
- Admin: Email/password login, invite page, users list with refresh
- Mobile: Phone login (returning), email invite → email OTP → phone OTP (new), home screen with user info
- Firebase: `@react-native-firebase/app` + `@react-native-firebase/auth` for native Phone Auth

## Epic 2: Request & Payout (Complete)

**Also delivered: Client-facing web app** — phone login, request advance, transaction history, terms acceptance, and role-based access control, all within the existing Next.js admin app. Uses the same backend APIs built for mobile.

### Step-by-step implementation order:

**1. Database — Add advance_requests table**
- Create migration: `000003_advance_requests.up.sql` with the table DDL from `docs/schema.md`
- Create matching `.down.sql`
- Run `go generate ./db/...` to regenerate sqlc models

**2. Backend — sqlc queries**
- Add `db/queries/advance_requests.sql`:
  - `CreateAdvanceRequest` — insert new request
  - `GetAdvanceRequestByID` — lookup by ID
  - `ListAdvanceRequestsByUserID` — user's request history
  - `ListAdvanceRequests` — admin view (all requests, ordered by created_at DESC, with pagination)
  - `UpdateAdvanceRequestStatus` — update status + optional failure_reason + payout_duration_seconds + campay_payout_ref

**3. Backend — Campay client**
- Create `internal/campay/client.go`:
  - `NewClient(permanentToken, baseURL, webhookSecret)`
  - `InitiateTransfer(phoneNumber, amount, description)` — POST to `POST /withdraw/`
  - `VerifyWebhook(token)` — JWT HS256 verification using `CAMPAY_WEBHOOK_SECRET`
- Add Campay config fields to `internal/config/config.go`: `CampayPermanentAccessToken`, `CampayWebhookSecret`, `CampayBaseURL`
- Wire client into `server.New()`

**4. Backend — Handlers & routes**
- `internal/handler/advance.go`:
  - `POST /api/advance-requests` — create request, call Campay, return request
  - `GET /api/advance-requests` — user's own request history (uses `RequireActiveUser` middleware)
  - `GET /api/admin/requests` — admin view (uses `RequireAdmin` middleware)
   - `POST /api/webhooks/campay` — public endpoint, verify JWT signature, update request status
- Wire routes in `server/server.go`:
  - Public: `POST /api/webhooks/campay`
  - User-protected: `POST /api/advance-requests`, `GET /api/advance-requests`
  - Admin-protected: `GET /api/admin/requests`

**5. Backend — Business logic**
- Before creating request: check user is active, has `is_terms_accepted = true`, no existing request with status `initiated` or `pending`
- On request creation: set `status = 'initiated'`, call Campay Withdraw API (`POST /withdraw/`)
- On Campay success (status `SUCCESSFUL`): set `status = 'success'`, record `payout_duration_seconds`
- On Campay failure (status `FAILED`): set `status = 'failed'`, record `failure_reason`
- Log `request_initiated`, `payout_success`, `payout_failed` events to `events` table

**6. Mobile — Terms screen**
- Add `app/(app)/terms.tsx` — display terms text, checkbox, "Accept" button
- On accept: call `POST /api/users/terms` (new endpoint) to update `is_terms_accepted`
- Add route to tabs or as a stack screen accessible from home/profile

**7. Mobile — Terms acceptance endpoint (Backend)**
- Add `PUT /api/users/terms` — update `is_terms_accepted = true`, `terms_accepted_at`, `terms_version`, `user_ip_at_consent`
- Add sqlc query: `UpdateTermsAcceptance` (already exists in `users.sql`)

**8. Mobile — Request flow**
- Add "Request Advance" button to `home.tsx`
- On tap: show confirmation modal (advance + charges will be deducted per terms)
- On confirm: call `POST /api/advance-requests`
- On success: navigate to history screen
- On error: show message (e.g., "Terms not accepted", "Request already in progress")

**9. Mobile — Transaction history**
- Replace empty `history.tsx` with real data: call `GET /api/advance-requests`
- Show list of requests with status badges, date, amount
- Auto-refresh or manual refresh button

**10. Admin — Requests page**
- Add `admin/src/app/(dashboard)/requests/page.tsx`
- Add `admin/src/hooks/use-requests.ts` hook
- Table: user email, amount (XAF), status badge, created date, payout ref, failure reason
- Add "Requests" to sidebar navigation
- Add `RequestStatus` to `admin/src/types/index.ts`

**11. Tests**
- Backend: unit tests for Campay client (mock HTTP), advance request handler, webhook handler
- Mobile: verify terms acceptance flow, request creation, history rendering
- Admin: requests page renders, status badges correct

## Epic 2.5: Firebase Removal — Own Auth (In Progress)

Remove Firebase from all frontends. Backend becomes sole auth authority using JWTs, bcrypt, and Africa's Talking SMS.

- Backend: `authjwt/` (JWT HS256), `authpassword/` (bcrypt), `sms/africastalking/` — all behind interfaces for microservice extraction
- Backend: new endpoints `POST /api/auth/send-phone-otp`, `POST /api/auth/verify-phone-otp`, `POST /api/auth/admin/login`, `POST /api/auth/refresh`, `POST /api/auth/logout`
- Backend: migration drops `firebase_uid`, adds `phone_otps`, `refresh_tokens`, `password_hash` on admins
- Backend: `cmd/create-admin` CLI tool for seeding admins
- Mobile: remove `@react-native-firebase/*`, use `expo-secure-store` for tokens, backend-driven OTP flow
- Admin: remove `firebase` web SDK, use localStorage for tokens, backend-driven auth

## Epic 3: Pilot Launch (Future)

- Kill switch toggle
- Request window enforcement (15th–end of month)
- Daily/monthly throttling
- OTP rate limiting (per-phone, e.g. max 3/hour for send-phone-otp and send-email-otp)
- Post-payout survey
- Payout speed metrics
- Push/SMS/email notifications
- Events log page
- E2E testing, production deployment
