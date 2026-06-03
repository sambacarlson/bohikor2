# Salary Advance Pilot App — Project Brief

## Overview

Bohikor2 is a salary advance pilot app. Employees can request a one-time 10,000 XAF advance, paid instantly via Campay mobile money.

## Current Scope (Epic 2): Request & Payout

Users who have completed authentication (Epic 1) can now request an advance and receive funds. Admins can monitor requests.

## Auth (Epic 1 — Complete, Epic 2.5 — Own Auth, Epic 3 — PIN Auth Overhaul)

- **Admin:** Email/password login → backend bcrypt verification → JWT access token (15min) + refresh token (30 days with rotation) → dashboard with Invite + Users
- **Mobile (New user):** Enter invited email → `GET /api/auth/check-invite` → `POST /api/auth/send-email-otp` → `POST /api/auth/verify-email-otp` (purpose=signup) → `POST /api/auth/create-pin` → home
- **Mobile (Returning user):** Enter email + 5-digit PIN → `POST /api/auth/login` → home
- **Mobile (Forgot PIN):** Enter email → `POST /api/auth/forgot-pin` → `POST /api/auth/verify-email-otp` (purpose=pin_reset) → `PUT /api/users/me/pin/reset` → home
- **Phone verification:** User adds phone in settings → `POST /api/users/phone` → Campay mini-withdrawal (1 XAF) → webhook confirms → phone marked verified
- **PIN rate limiting:** 3 failed attempts/hour, then 1hr cooldown, then 3 more attempts → account locked. Locked accounts unlocked by admin via `PUT /api/admin/users/:id/unlock`
- **Tokens:** HS256 JWT access tokens (15min) + opaque refresh tokens (30 days) with rotation. Stored in expo-secure-store (mobile) or localStorage (admin).
- **No Firebase, no SMS dependency.** Backend is sole auth authority. Phone verification uses Campay mini-withdrawal instead of SMS OTP.

## Request Flow (Epic 2 — Complete)

### Mobile

1. User taps "Request Advance" on home screen
2. Confirmation modal: informs user that advance + charges will be deducted from salary per terms
3. Backend checks: user exists, is active, has accepted terms, no active request in progress
4. Request created with `status = 'initiated'`
5. Backend calls Campay Withdraw API (`POST /withdraw/`) to send 10,000 XAF to user's phone number
6. Campay processes payout → sends webhook to backend
7. Backend verifies webhook JWT signature (HS256, embedded in body), updates request status (`pending` → `SUCCESSFUL` or `FAILED`)
8. User sees request in transaction history with live status

### Eligibility (for now)

- User must have verified phone AND accepted terms before requesting advance
- No existing request with status `initiated` or `pending` (prevent duplicate in-flight requests)
- ~~Request window 15th–end of month~~ — **deferred**
- ~~Daily attempt limit~~ — **deferred**
- ~~Monthly success limit~~ — **deferred**

### Terms Handling

- Terms are accepted via a dedicated screen in the mobile app (separate from auth)
- Stored on `users` table: `is_terms_accepted`, `terms_accepted_at`, `terms_version`, `user_ip_at_consent`
- Before any advance request, backend checks `is_terms_accepted = true`
- If not accepted, return error prompting user to accept terms first
- Terms text is hardcoded in the mobile app for now (no versioning system needed yet)

### Admin Dashboard

- **Requests page:** Table showing all advance requests — user email, amount, status, created date, payout reference, failure reason
- **Users page:** Already exists (from Epic 1)
- **Invite page:** Already exists (from Epic 1)

## Campay Integration

- **Withdraw API** (`POST /withdraw/`) — send money to user's mobile money wallet (status: `SUCCESSFUL`, `FAILED`, `PENDING`)
- **Webhook** — receive payout status updates (JWT HS256 signed, embedded as `signature` field in body)
- **JWT verification** — verify webhook signatures using `CAMPAY_WEBHOOK_SECRET` via `golang-jwt`
- **Auth** — permanent access token (`Authorization: Token <token>`)
- All credentials configured in backend `.env`

## Future (Deferred)

- Kill switch (block new requests, flag in-flight for review)
- Request window enforcement (15th–end of month, Africa/Douala)
- Daily attempt limit (1/day), monthly success limit (1/month)
- Post-payout satisfaction survey
- Payout speed metrics (P50/P90)
- Push/SMS/email notifications
- Events log page on admin dashboard
- Admin management page
