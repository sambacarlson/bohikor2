# Known issues

## - [ ] `sql.NullTime` fields serialize as `{Time, Valid}` objects, not plain nullable strings

Any sqlc-generated Go field typed `sql.NullTime` (the mapping CLAUDE.md documents for nullable
`timestamptz` columns) has no custom JSON marshaling, so it serializes as a nested object —
`{"Time": "2026-01-01T00:00:00Z", "Valid": true}` when set, `{"Time":
"0001-01-01T00:00:00Z", "Valid": false}` when null — never a plain ISO string or `null`. Verified
empirically (`go run` a small marshal test against `database/sql.NullTime` directly). This is
unlike every `pgtype.*` type used elsewhere in this codebase (`pgtype.Text`, `pgtype.UUID`,
`pgtype.Numeric`), which do implement clean JSON marshaling (plain string/number, or `null`) —
also verified empirically.

Confirmed affected fields: `AdvanceRequest.last_reconciled_at` / `next_retry_at` (exposed by the
new `GET /api/platform/requests/needs-review` endpoint), and `User.terms_accepted_at` /
`locked_until` (exposed via `sanitizeUser()` in `internal/handler/auth.go` and the raw
`GET /api/users/me` response). Any nullable-timestamptz column follows the same pattern —
this list is what's actually reachable from the frontend today, not necessarily exhaustive.

**Recommended fix:** either (a) have `db/sqlc.yaml` override nullable `timestamptz` columns to
`pgtype.Timestamptz` instead of the stdlib `sql.NullTime` (pgtype's version already marshals
correctly, matching the pattern already used for `pgtype.Text`/`pgtype.UUID`/`pgtype.Numeric`), or
(b) add explicit conversion in each handler that returns one of these fields (mirroring the
existing `numericToString()` helper in `internal/handler/platform.go` for `pgtype.Numeric`).

**Not currently blocking anything** — `complete-epic-8.md` was corrected (2026-08-05) to type
these fields as `unknown` on the frontend and explicitly instructs never rendering them, so no
planned Epic 8 screen depends on this being fixed. Worth fixing before any future feature actually
needs to display one of these timestamps (e.g. "reconciled 3 attempts ago, last at...").

Found 2026-08-05 while verifying the two new backend endpoints (`GET /api/admin/ledger`,
`GET /api/platform/requests/{needs-review,health}`) for the Epic 8 frontend plan.

## - [ ] `amount_xaf` fields serialize as JSON numbers but are typed `string` in the frontend

`pgtype.Numeric` fields returned raw (i.e. not pre-converted via `numericToString()`, which
`balance_xaf` always is) serialize as a plain JSON **number** — e.g. `15000.50`, not `"15000.50"`
— verified empirically. This affects every `amount_xaf` field returned as part of an
`advance_requests` or `company_ledger` row: `AdvanceRequest.amount_xaf` (all advance-request
endpoints, pre-existing — not introduced by this session's work) and
`LedgerEntry.amount_xaf` (new, `GET /api/admin/ledger`'s `entries` array). `bohikor/src/types`
has always typed `AdvanceRequest.amount_xaf` as `string`, which doesn't match the actual wire
value; nothing has broken yet because JSX/template-literal interpolation coerces either type to
the same displayed text, but any code calling a string-only method on it (`.startsWith()`,
`.includes()`, etc.) will throw at runtime — this nearly shipped as a bug in `complete-epic-8.md`
step 18's original "red text if the value starts with `-`" instruction, caught and fixed
2026-08-05 to use `Number(entry.amount_xaf) < 0` instead.

**Recommended fix:** for currency-precision safety (floating-point JSON numbers are not a safe
representation for money), convert every `amount_xaf`-bearing response to a string server-side,
the same way `balance_xaf` already is via `numericToString()` — either add the same conversion to
each handler that returns raw `AdvanceRequest`/`CompanyLedger` rows, or (bigger change) have sqlc
map `numeric` columns to a wrapper type with custom string-producing JSON marshaling instead of
raw `pgtype.Numeric`.

**Not blocking** — `complete-epic-8.md`'s Reference section now has an explicit "wire-shape
gotchas" note instructing safe (type-agnostic) handling of this field everywhere it's used.

Found 2026-08-05, same verification pass as above.

## - [ ] `FRONTEND_BASE_URL` defaults to `http://localhost:3000` with no deploy-checklist entry

`backend/internal/config/config.go`'s `FrontendBaseURL` (added for the invitation-email signup
link) defaults to `http://localhost:3000` if the env var isn't set. `AGENTS.md`'s deploy/pre-push
checklist doesn't mention this variable. If it's forgotten when configuring the production
environment (Render), every invitation email sent from production will link to a broken
`localhost` URL instead of the real deployed frontend — a silent failure that would only surface
when an invited employee actually clicks the link.

**Recommended fix:** add `FRONTEND_BASE_URL` to `AGENTS.md`'s deploy pre-push checklist, and/or
remove the `localhost` default in production builds so a missing value fails loudly (e.g. `Config`
validation error) rather than silently producing broken links — mirroring how `DatabaseURL` is
already required with no silent fallback in `config.Load()`.

**Not blocking** — purely an operational/deploy-config concern, unrelated to any frontend
implementation work in `complete-epic-8.md`.

Found 2026-08-05, same verification pass as above.

## - [ ] `GET /api/users/me` leaks the employee's bcrypt PIN hash and lockout fields

`handleUserMe` in `backend/internal/server/routes.go` (lines 17-41) returns the raw `db.User`
struct (`c.JSON(http.StatusOK, gin.H{"data": user})`). `db.User.PinHash` is tagged
`json:"pin_hash"` with no redaction, so every response from this endpoint includes the employee's
bcrypt PIN hash in plaintext JSON, along with `failed_login_attempts`, `locked_until`, and
`user_ip_at_consent`.

Same root cause as the `GET /api/admin/me` leak fixed just below (and the same class of bug the
`AdminLogin`/`sanitizeAdmin` fix addressed on the admin side) — but never applied to the
employee-facing equivalent. `internal/handler/auth.go` already has a `sanitizeUser()` helper
(lines 800-815, used by `Login`/`CreatePin`/`VerifyEmailOTP`) that returns an explicit allowlist
(id/email/email_verified/full_name/phone_number/phone_verified/status/is_terms_accepted/
terms_accepted_at/terms_version/created_at/updated_at) excluding `pin_hash` and the lockout
fields — `handleUserMe` should use it (or an equivalent) instead of serializing the raw struct.

Found 2026-08-05 while scoping the Epic 8 frontend plan's employee `use-user` hook, which calls
this endpoint. Not fixed yet — logged here per instruction rather than fixed inline.

## - [x] `GET /api/admin/me` leaks the admin's bcrypt password hash

`handleAdminMe` in `backend/internal/server/routes.go` returns the raw `db.Admin` struct
(`c.JSON(http.StatusOK, gin.H{"data": admin})`). `db.Admin.PasswordHash` is tagged
`json:"password_hash"` with no redaction, so every response from this endpoint includes the
admin's bcrypt password hash in plaintext JSON.

Same root cause as the `AdminLogin` password-hash leak fixed on `epic-6-multi-tenant-foundation`
(PR #2) — that fix added a `sanitizeAdmin()` helper in `backend/internal/handler/auth.go` that
strips `PasswordHash` before the admin record goes into a response body. `handleAdminMe` lives in
`internal/server`, not `internal/handler`, and was out of scope for that PR's diff, so it still
needs the same fix: use `sanitizeAdmin(admin)` (or an equivalent) instead of the raw struct.

**Fixed:** `handleAdminMe` now builds an explicit `gin.H` (id/company_id/email/created_at) instead
of serializing the raw `db.Admin` struct, so `PasswordHash` is never marshaled. Regression test:
`TestAdminMeEndpoint_ActiveAdmin` in `backend/internal/server/server_test.go`.

# Possible problems/questions to address

## [ ] account locked on multiple attempts may allow bad actors to intentionally block legitimate user accounts

In case a bad actor lays hands on user phone number, they may attempt otps multiple times to create a form of DOS for actual user.
A possible fix could be to collect device info such as IP addr and browser id or mac addrr of user to block those in certain scenarios and the account itself on other scenarios. We could also use this to request for opt by email again in case device or location change looks fishy.
Still random thoughts. yet to evaluate clearly and see if this is a valid problem or how best to address it.

# phone verification has no reconciler coverage. ListReconcilableAdvanceRequests

only queries advance_requests; a phone verification stuck in pending with a lost webhook has no
automatic recovery path in this codebase today — it relies entirely on the webhook arriving.
What are some ways to fix this? this is necessary.
