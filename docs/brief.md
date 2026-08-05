# Bohikor — Project Brief

## Overview

Bohikor is a multi-tenant salary-advance platform. Employees of a subscribed company can request
a salary advance, paid instantly via Campay mobile money, against their employer's own funded
float. A platform super-admin onboards companies and funds them; each company's own admins manage
their employees and monitor requests day to day.

## Tenancy & Roles

- **Platform admin** (global, belongs to no company) — provisions companies and their first
  company admin, sets/tops up each company's float balance, suspends/activates companies, and has
  a cross-company view of stuck/needs-review payouts.
- **Company admin** (scoped to one company) — invites employees, manages per-company settings
  (kill switch, request window, daily/monthly limits, advance amount), views the company's
  ledger/balance (view-only — top-ups are platform-admin only), and reconciles/resolves/reissues
  stuck or failed requests.
- **Employee** (scoped to one company; email is **globally unique** across the whole platform) —
  requests advances, views request history, manages phone verification and PIN.

## Auth

- **Platform admin:** email/password → `/platform` (super-admin console)
- **Company admin:** email/password → `/{company}/admin`
- **Employee (returning):** email + 5-digit PIN → `POST /api/auth/login`. Login resolves the
  employee's company from their (globally unique) email and returns `company_slug` for the
  `/{slug}` redirect.
- **Employee (new):** invited email → `GET /api/auth/check-invite` → `POST /api/auth/send-email-otp`
  → `POST /api/auth/verify-email-otp` (purpose=signup) → `POST /api/auth/create-pin`
- **Forgot PIN:** email → `POST /api/auth/forgot-pin` → OTP verify → `PUT /api/users/me/pin/reset`
- **Phone verification:** Campay Collect API (`POST /collect/`) debits a small configurable amount
  from the employee's phone; they dial the returned USSD code and enter their mobile-money PIN; a
  webhook confirms and marks the phone verified. No SMS dependency.
- **PIN rate limiting:** 3 failed attempts/hour → 1hr cooldown → 3 more attempts → account locked;
  a company admin unlocks via `PUT /api/admin/users/:id/unlock`.
- **Tokens:** HS256 JWT access tokens (15min) + opaque rotating refresh tokens (30 days). Claims
  carry `role` (`user | admin | platform_admin`) and `company_id` (empty for platform admins).
  Backend is the sole auth authority — no third-party auth provider.

## Request & Payout Flow

1. Employee taps "Request Advance".
2. Backend checks eligibility: user active, phone verified, terms accepted, no in-flight request,
   company kill switch off, within the company's request window, under its daily/monthly limits,
   and **company float ≥ the advance amount**.
3. In one DB transaction (row-locked on the company for concurrency safety): create the request
   (`initiated`) and post a `payout_debit` ledger entry reserving the float.
4. Call Campay's Withdraw API. Outcome maps to a status: `SUCCESSFUL`/`PENDING` →
   `success`/`pending`; a Campay-declined transfer → `failed` (+ ledger reversal); a timeout or
   transport error (no response at all) → `processing` — **never** guessed as `failed`, since the
   whole point of `processing` is that we don't yet know if money moved.
5. Async confirmation arrives via a JWT-signed webhook. A background reconciler also polls
   `processing`/`pending` requests on a backoff schedule (30s → 15min), so a lost webhook or a
   timeout doesn't strand a request; after repeated failures a request is escalated to
   `needs_admin_review`.
6. Recovery: an employee can retry a terminally `failed` request (a fresh attempt, new idempotency
   key); a company admin can force a reconcile, mark a review resolved, or reissue a payout.

## Company Ledger & Float

- `company_ledger` is an append-only, immutable table (`topup`, `payout_debit`, `reversal`,
  `adjustment` entries); a company's balance is the running sum of its entries.
- There is **one platform-level Campay account**; each company draws its payouts from its own
  float balance, not a shared pool.
- The platform admin is the sole funder — company admins see their balance and ledger history but
  cannot request or post top-ups themselves.

## Onboarding

1. Platform admin creates a company (slug + name), its first company admin, and sets its starting
   balance.
2. That company admin invites employees by email; the invitation email links to
   `/{company}/signup?email=…` with the company slug embedded.
3. Company self-signup is deferred — onboarding a new employer is currently a platform-admin action.

## App Structure

One Next.js app (`bohikor/`), path-routed by tenant:

- `/` — landing + login, resolves the employee's company from their email and redirects
- `/{company}` — employee app (request advance, history, account/phone/PIN settings)
- `/{company}/admin` — company admin (users, invite, requests + reconcile/resolve/reissue,
  settings, balance/ledger, events)
- `/platform` — platform super-admin (companies, provisioning, balance top-ups, cross-company
  request health)

`mobile/` (Expo) is **frozen** — kept for reference, no longer the primary employee client; see
`mobile/README.md`.

## Campay Integration

- **Collect API** (`POST /collect/`) — request payment from an employee's mobile money wallet
  (returns a `ussd_code` the user must dial; always async). Used for phone verification.
- **Withdraw API** (`POST /withdraw/`) — send money to an employee's mobile money wallet
  (`SUCCESSFUL`, `FAILED`, `PENDING`). Used for advance payouts.
- **Status lookup** (`GET /transaction/{ref}/`) — used by the reconciler to poll an unconfirmed
  payout by its idempotent `external_reference` (the request's own UUID).
- **Webhook** — receives async payout/collect status updates (JWT HS256 signed, embedded as a
  `signature` field in the body), verified via `CAMPAY_WEBHOOK_SECRET`.
- **Auth** — one platform-level permanent access token (`Authorization: Token <token>`), configured
  once in the backend's environment; not per-company.

## Terms Handling

- Terms are accepted via a dedicated screen, separate from auth.
- Stored on `users`: `is_terms_accepted`, `terms_accepted_at`, `terms_version`, `user_ip_at_consent`.
- Checked before any advance request; an unaccepted user is prompted to accept terms first.

## Future (Deferred)

- Company self-signup (currently platform-admin provisioned only)
- Post-payout satisfaction survey
- Payout speed metrics (P50/P90)
- Push/SMS/email notifications
- Company-admin-requested top-ups (currently platform-admin only, view-only for company admins)
