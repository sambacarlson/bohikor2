# Known issues

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
