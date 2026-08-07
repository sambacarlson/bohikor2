# Known issues

Ordered by priority for execution. Each item was re-verified 2026-08-07 against current code
(agents cited file:line) before being carried forward — see each entry's "Verified" line.

## - [x] 1. `GET /api/users/me` leaks the employee's bcrypt PIN hash and lockout fields

`handleUserMe` (`backend/internal/server/routes.go:17-41`) returns the raw `db.User` struct via
`c.JSON(http.StatusOK, gin.H{"data": user})`. `db.User.PinHash` (`db/sqlc/models.go:325`) has no
redaction tag, so every response includes the bcrypt PIN hash plus `failed_login_attempts`,
`locked_until`, `user_ip_at_consent`. `sanitizeUser()` (`internal/handler/auth.go:800-815`)
already exists as the allowlisted fix (used by Login/CreatePin/VerifyEmailOTP) but is never called
from `routes.go`.

**Fix:** route `handleUserMe` through `sanitizeUser()` (or equivalent) instead of the raw struct.

**Verified 2026-08-07: still valid**, confirmed at the cited lines.

**Resolved 2026-08-07 (#19):** exported `sanitizeUser()` as `handler.SanitizeUser` and routed
`handleUserMe` through it; regression test asserts the leaked fields never appear in the response.

## - [x] 2. `sql.NullTime` fields serialize as `{Time, Valid}` objects, not plain nullable strings

`db/sqlc.yaml:24-29` maps nullable `timestamptz` → `database/sql.NullTime`, which has no custom
`MarshalJSON`. Confirmed affected fields (`db/sqlc/models.go`): `AdvanceRequest.LastReconciledAt`/
`NextRetryAt` (211-212, exposed via `GET /api/platform/requests/needs-review`),
`User.LockedUntil`/`TermsAcceptedAt` (327, 330), `PhoneVerification.BlockedUntil` (252),
`Invitation.AcceptedAt` (275). Scope is **broader than originally logged**: `TermsAcceptedAt` also
leaks this shape through `sanitizeUser()` itself (`auth.go:800-815`), so even the "sanitized"
Login/CreatePin/VerifyEmailOTP responses carry it — not just the raw `/me` endpoint from issue 1.
Unlike `pgtype.*` types used elsewhere (`pgtype.Text`, `pgtype.UUID`, `pgtype.Numeric`), which
marshal cleanly.

**Fix:** override nullable `timestamptz` → `pgtype.Timestamptz` in `db/sqlc.yaml` (marshals
correctly, consistent with the rest of the schema) rather than patching every handler individually.

**Verified 2026-08-07: still valid, scope expanded.**

**Resolved 2026-08-07 (#21):** overrode `db/sqlc.yaml`'s nullable `timestamptz` mapping to
`pgtype.Timestamptz` (confirmed empirically to marshal as a plain ISO string or `null`), updated
every call site, and widened the frontend's defensively-`unknown` fields to `string | null`. Also
fixed a live bug this uncovered: `GET /api/admin/users`' `locked_until` had been rendering
`Invalid Date` since the frontend `AdminUser` type already (incorrectly, until now) assumed the
correct shape.

## - [x] 3. `amount_xaf` fields serialize as JSON numbers but are typed `string` in the frontend

`AdvanceRequest.AmountXaf`, `LedgerEntry.AmountXaf` (`db/sqlc/models.go:205,233,284`) are
`pgtype.Numeric`, returned raw (not through `numericToString()`, which only `balance_xaf` uses —
`internal/handler/ledger.go:54`, `platform.go:110,150,287,344`). `pgtype.Numeric` marshals as a
JSON number, e.g. `15000.50`. `bohikor/src/types/index.ts:85,106,142` still types these fields
`string` — a currency-precision mismatch (unsafe for money) that hasn't broken yet only because
JSX interpolation coerces either type to the same displayed text.

**Fix:** convert every `amount_xaf`-bearing response to a string server-side, same pattern as
`numericToString()` for `balance_xaf`.

**Verified 2026-08-07: still valid.**

**Resolved 2026-08-07 (#23):** the only NUMERIC columns in the entire schema are the three
`amount_xaf` columns (confirmed by grepping the migration), so rather than patching every handler,
overrode `db/sqlc.yaml`'s `numeric` mapping globally to a new `dbtypes.NumericString` wrapper
(`backend/internal/dbtypes/numeric_string.go`) that marshals as a JSON string (`null` when
invalid) and otherwise behaves exactly like `pgtype.Numeric` (Scan/Value promoted via embedding).
Also discovered the sqlc.yaml `db_type: "numeric"` override had never actually matched (needed the
fully-qualified `pg_catalog.numeric`), so the project's stated `numeric` → `decimal.Decimal`
mapping had silently never applied to anything — unrelated dead config, now replaced. Frontend
needed no changes; `amount_xaf` was already typed `string` there.

## - [x] 4. `FRONTEND_BASE_URL` defaults to `localhost`, breaking invite emails in production

`backend/internal/config/config.go:33` defaults `FrontendBaseURL` to `http://localhost:3000` with
no required-value validation (unlike `DatabaseURL`, `config.go:52-54`). Used in
`internal/service/invite.go:87` to build the signup link embedded in every invitation email
(`internal/email/email.go:28-52` — confirmed there is **no separate hardcoded-localhost bug** in
the email template itself; it's entirely this one config default flowing through). `AGENTS.md`'s
deploy checklist doesn't mention this variable. If forgotten in production (Render), every invite
email links to a broken `localhost` URL — silent until an invited employee clicks it. (Merges what
was a duplicate bullet buried in the old "manage company form" mega-issue — same root cause.)

**Fix:** add `FRONTEND_BASE_URL` to `AGENTS.md`'s deploy checklist, and make it a required config
value (fail loudly at startup) rather than silently defaulting in non-dev environments.

**Verified 2026-08-07: still valid.**

**Resolved 2026-08-07 (#24):** removed the `envDefault` from `FrontendBaseURL`; `config.Load()` now
falls back to `http://localhost:3000` outside production but returns a startup error if it's unset
in production. Added a pre-push checklist item to `AGENTS.md` reminding to verify the Render value
is the real deployed URL (the code only catches "unset," not "set to a stale/wrong URL").

## - [ ] 5. Account/OTP lockout has no IP or device throttling — enables targeted DOS

PIN login (`internal/handler/auth.go:184-210`): 3 failed attempts → 1hr lock, 6 → permanent
`UserStatusLocked` — per-account only. OTP failures (`checkEmailOTPBlocked`, `auth.go:434-455`) via
`email_otp_failures` table — same pattern, temporary then permanent block. **Both are keyed purely
by email/account**, no IP or device fingerprint anywhere. Anyone who knows a victim's email/phone
can lock them out — confirmed real, matching the user's original suspicion.

**Needs design** — brainstorm before implementing (candidates: IP/device-based secondary
throttling, CAPTCHA after N attempts, re-verification via email on suspicious device/location
change). Flagged by the user as worth solving but not yet scoped.

**Verified 2026-08-07: confirmed valid DOS vector.**

## - [x] 6. No checks against self-invite or cross-company/cross-role email reuse

`internal/service/invite.go` `Invite()` (54-101) only checks for an existing *pending* invitation
to the same email (`GetInvitationByEmail`, 67-71) — never checks the `users` or `admins` tables,
never compares against the inviting admin's own email. `admins.email` and `users.email` are
independently unique (`backend/migrations/000001_schema.up.sql:46,59`), so the same email can be
an admin in one company and a user in another (or even the same company) simultaneously, and an
admin can invite themself as an employee.

**Needs a policy decision** (user's stated preference: should not be possible) before implementing
the check — brainstorm exact rules (globally unique across `users`+`admins`? per-company only? can
one person hold both an admin and employee role by design?).

**Verified 2026-08-07: still valid, zero checks exist.**

**Resolved 2026-08-07 (#25):** policy decided — email is one identity across the whole app; an
email already in `users` or `admins` (any company) blocks both a new invite (`Invite()` in
`internal/service/invite.go`, which naturally also blocks an admin inviting their own email since
that email is already an `admins` row) and platform-admin `CreateCompanyAdmin`. The reverse case
(same admin email in two companies) was already blocked pre-existing by `admins.email`'s DB-level
unique constraint. Frontend invite-error handling was also fixed: it previously assumed every 409
meant "duplicate invitation" and would have shown a misleading message for this new case — now
uses the shared `getApiErrorMessage()` helper to surface the backend's actual message.

## - [x] 7. Login never verifies the URL's company slug server-side

`POST /login` (`server.go:99`) request struct only has `Email`/`PIN`
(`auth.go:142-145`) — no slug field. Company is resolved *after* authentication purely from
`user.CompanyID` (`auth.go:213`, comment: "email is globally unique") and returned as
`company_slug` for the frontend to redirect with. The `{company}` slug in the URL today is a pure
frontend routing artifact, never cross-checked against the authenticated user's actual company.

**Fix:** have `Login()` accept/require the slug from the request path and reject if it doesn't
match the resolved user's company — closes the gap the user flagged ("verify company id or slug
matches... before login").

**Verified 2026-08-07: still valid.**

**Resolved 2026-08-07 (#26):** `Login()` and `AdminLogin()` now require `company_slug` in the
request body and reject (same generic `invalid_credentials` message as a wrong PIN/password — a
slug mismatch can't be used to probe which company an email belongs to) if it doesn't match the
resolved user's/admin's actual company. Reuses the `GetCompanyByID` call each handler already made
post-auth, so no extra query. `PlatformLogin` is untouched (no company concept). Frontend: both
`[company]/login` and `[company]/admin/login` now send the URL's `{company}` param as
`company_slug`.

## - [x] 8. Phone verification has no reconciler coverage

Phone verification is genuinely async/webhook-based (Campay USSD push), not synchronous OTP:
`AddPhoneNumber` (`internal/handler/phone.go:43-187`) creates a `phone_verifications` row
(`status='initiated'` → `'pending'`, storing `campay_payout_ref`), resolved only by
`handlePhoneVerificationWebhook` (`internal/handler/advance.go:743-786`). The
`phone_verifications` table (`migrations/000001_schema.up.sql:174-190`) has **no**
`attempt_count`/`next_retry_at`/`last_reconciled_at`/`needs_admin_review` columns, unlike
`advance_requests`. `ListReconcilableAdvanceRequests` (`db/queries/advance_requests.sql:52-70`)
and the reconciler (`internal/reconciler/reconciler.go`) touch only `advance_requests` — zero
references to `phone_verifications` anywhere in the reconciler. A lost webhook leaves a row stuck
in `pending` forever; `GetActivePhoneVerificationByUser` then blocks the user from retrying with a
409 (`phone.go:80-84`). No resend/cancel/timeout path exists at all.

**Fix:** extend the reconciler pattern to `phone_verifications` — needs schema columns
(attempt/backoff tracking) mirroring `advance_requests`, plus a resend/cancel path so a stuck
verification doesn't permanently lock the user out.

**Verified 2026-08-07: still valid** (dedicated investigation, confirmed webhook-only with no
fallback).

**Resolved 2026-08-07 (#27):** added the same `attempt_count`/`last_reconciled_at`/`next_retry_at`/
`needs_admin_review` columns to `phone_verifications` and extended `Reconciler.Tick` to also poll
and resolve stuck rows via `ListReconcilablePhoneVerifications`/`UpdatePhoneVerificationReconcileAttempt`
(parallel to the `advance_requests` path, but without `service.TransitionRequest`'s ledger/
transaction machinery — phone verification never touches the company ledger, matching
`handlePhoneVerificationWebhook`'s existing plain-update pattern). For the "resend" half: found
`GetActivePhoneVerificationByUser` had no age bound at all, while the frontend's `canRetry` UX
already assumed a stuck verification could be retried after 60s — a pre-existing mismatch, not just
this issue's gap. Fixed by age-bounding that query to 60s, which is both simpler and faster than
waiting on `needs_admin_review` (~23min backoff ladder) and finally makes the frontend's existing
retry affordance actually work. No frontend changes needed.

## - [x] 9. Some 500 responses don't log the underlying error (rescoped from "no debug logs")

**Rescoped — the premise of "no logging at all" is stale.** Structured logging already exists:
`middleware.Logger()` (`internal/middleware/middleware.go:10-26`) logs every request via
`slog.Info` (method/path/status/latency/ip), wired in `server.go:76-78` alongside
`gin.Recovery()` and `RequestID()`. 59 `slog.Error`/`slog.Warn` call sites exist across handlers.
**What's actually still true:** many `JSONError(..., http.StatusInternalServerError, ...)` sites
have no adjacent `slog.Error` logging the real Go `err` (e.g. `auth.go:215,229,278,288,303,323`),
so some 500s only ever surface the generic client-facing message on stdout — the exact frustration
the user described, just narrower in scope than "no logs exist."

**Fix:** audit every `JSONError(c, http.StatusInternalServerError, ...)` call site across
`internal/handler/*.go` and ensure each logs the underlying error via `slog.Error` before
responding.

**Verified 2026-08-07: partially valid, rescoped to the actual gap.**

**Resolved 2026-08-07 (#28):** audited all 76 `JSONError(..., http.StatusInternalServerError, ...)`
call sites across `internal/handler/*.go` with a script, then by hand. Added `slog.Error` (with the
real `err` plus relevant IDs — user/admin/company id, email — for grep-ability) at the 41 sites that
were genuinely silent, including 5 `user_id`-type-assertion invariant violations and the shared
`companyIDFromContext` helper. The 3 remaining flagged sites (`advance.go:365`, `admin_requests.go:202`,
`phone.go:168`) turned out already covered — `disburseAndResolve`/the collect-failure fallback log
internally on every path that reaches them. Also converted one stray `fmt.Printf("WARN: ...")` in
`auth.go` (post-signup `AcceptInvitation` failure) to `slog.Warn`, same class of bug.

## - [x] 10. "platform" isn't reserved as a company slug; no slug-existence check before login renders

`slugPattern` (`internal/handler/platform.go:19`, mirrored in
`bohikor/src/app/platform/(protected)/page.tsx:41`) is a character-format regex only — no reserved-
word blocklist, so a company could be created with slug `platform` and collide with the static
`/platform` route. Separately, `bohikor/src/app/[company]/login/page.tsx` renders the login form
for **any** `[company]` value with no existence check first — `GetCompanyBySlug` exists only as a
sqlc query (`db/sqlc/companies.sql.go:65`) used by the `create-admin` CLI, never exposed over HTTP.

**Scope decision (user confirmed): minimal fix only, no `/org/{slug}` route restructure.**
- Backend: expose a public `GET /api/companies/by-slug/:slug`, and blocklist reserved words
  (`platform`, and any other top-level route segments) on company creation.
- Frontend: `[company]/login` calls the new endpoint first and shows a "company not found" state
  instead of a login form when the slug doesn't resolve.

**Verified 2026-08-07: still valid**, scope narrowed per user decision.

**Resolved 2026-08-07 (#29):** `CreateCompany` now rejects a `reservedSlugs` blocklist (currently
just `platform`) with 400 `reserved_slug`. Added a public `GET /api/companies/by-slug/:slug`
(`internal/server/routes.go`'s `handleGetCompanyBySlug`, matching the existing inline-handler
pattern used for `/me`) returning only `{slug, name}` — no balance/status/other data a logged-out
visitor shouldn't see. Both `[company]/login` and `[company]/admin/login` now call it via a new
`useCompanyBySlug` hook before rendering: a "Company not found" card replaces the login form for an
unresolved slug, and as a side benefit (the data was already being fetched) the card title now
shows the company's real name instead of the raw slug.

## - [ ] 11. Manage-company UI is missing admin visibility, copy-link affordances, and nav reorg

`CompanyDetail` (`bohikor/src/app/platform/(protected)/page.tsx:43-345`) is already a Radix
`Dialog` with a working double-confirm pattern for suspend (lines 219-243) — directly reusable for
admin-credential actions. What's missing, confirmed absent:
- No list of a company's existing admins (no GET-admins endpoint/hook exists — `use-companies.ts`
  only has a POST-admin mutation).
- No copy-link affordance for the company URL or admin creds.
- Reconciliation health/needs-review are not on their own nav page.
- No self-service admin password-reset entry point (see issue 12).

**Needs design** before implementation — this is a bundle, not a single fix; brainstorm the exact
UI shape (dock modal vs. keep as dialog, where copy-link lives, nav structure) before coding.

**Verified 2026-08-07: still valid, confirmed via UI inspection.**

## - [ ] 12. Company admins have no self-service password reset

Grep for `change-password`/`reset-password`/`changePassword`/`resetPassword` across
`bohikor/src` and `backend/internal` returns nothing. The only password-adjacent capability is
platform-admin-initiated "Create Admin" (sets an initial password). Employees have a PIN
reset flow (`forgot-pin`/`reset-pin`) but that's a different flow/subject entirely. Company admins
have no old-password/new-password/confirm flow anywhere.

**Fix:** add a company-admin password-change endpoint (old password + new password + confirm) and
a nav entry to reach it — likely paired with issue 11's nav work.

**Verified 2026-08-07: still valid, confirmed missing.**

## - [ ] 13. Inline error messages are easy to miss (rescoped from "user-facing errors not up to par")

**Rescoped — the plumbing already exists and is more solid than the original issue assumed.**
Sonner toasts + inline `<Alert variant="destructive">` + per-endpoint error-code mapping (e.g.
`invalid_credentials`, `account_suspended`, `company_suspended`) are already wired consistently
across every login/invite form checked (`[company]/login/page.tsx:39-81`,
`[company]/admin/login/page.tsx:39-60`, `platform/login/page.tsx:38-59`,
`[company]/admin/(protected)/invite/page.tsx:28-55`). A shared `getApiErrorMessage()` helper
exists (`bohikor/src/lib/api.ts:108-115`) but isn't used everywhere — some pages inline their own
extraction instead.

**What's actually still true, per user's own example:** a wrong PIN on login renders its error
above the email field in a style that's easy to miss — a *visual prominence/placement* problem,
not a missing-plumbing problem. Design and payout-failure visibility (does a failed payout clearly
surface as an error to the employee, not just silently sit in a status field?) both need a look
before deciding the fix — brainstorm the exact prominence/placement change (position relative to
the field that caused it, color/weight, whether critical failures like payout warrant something
stronger than a toast) rather than jumping straight to implementation.

**Verified 2026-08-07: infrastructure is solid; rescoped to prominence/placement + consistency.**

# Resolved — verified against current code, not carried into the execution queue

## - [x] `event.preventDefault()` on forms

Checked `[company]/login`, `[company]/admin/login`, `platform/login`, and the invite form — all
four call `e.preventDefault()` as the first line of their submit handler
(`login/page.tsx:40`, `admin/login/page.tsx:40`, `platform/login/page.tsx:39`,
`invite/page.tsx:29`). No full-page-reload-on-failed-submit reproduced anywhere checked.

**Verified 2026-08-07: resolved / not reproduced.** Re-open with a specific repro if one turns up.

# Possible problems/questions carried forward without a validity check yet

(none remaining — all 8 original items above were evaluated and folded into the numbered list or
closed.)
