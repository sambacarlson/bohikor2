# Known issues

## - [ ] `GET /api/admin/me` leaks the admin's bcrypt password hash

`handleAdminMe` in `backend/internal/server/routes.go` returns the raw `db.Admin` struct
(`c.JSON(http.StatusOK, gin.H{"data": admin})`). `db.Admin.PasswordHash` is tagged
`json:"password_hash"` with no redaction, so every response from this endpoint includes the
admin's bcrypt password hash in plaintext JSON.

Same root cause as the `AdminLogin` password-hash leak fixed on `epic-6-multi-tenant-foundation`
(PR #2) — that fix added a `sanitizeAdmin()` helper in `backend/internal/handler/auth.go` that
strips `PasswordHash` before the admin record goes into a response body. `handleAdminMe` lives in
`internal/server`, not `internal/handler`, and was out of scope for that PR's diff, so it still
needs the same fix: use `sanitizeAdmin(admin)` (or an equivalent) instead of the raw struct.
