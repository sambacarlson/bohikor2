-- name: GetEmailOTPFailure :one
SELECT * FROM email_otp_failures WHERE email = $1;

-- name: UpsertEmailOTPFailure :one
INSERT INTO email_otp_failures (email, consecutive_failures, last_failure_date, blocked_until, is_permanently_blocked, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (email) DO UPDATE SET
    consecutive_failures = EXCLUDED.consecutive_failures,
    last_failure_date = EXCLUDED.last_failure_date,
    blocked_until = EXCLUDED.blocked_until,
    is_permanently_blocked = EXCLUDED.is_permanently_blocked,
    updated_at = NOW()
RETURNING *;

-- name: ResetEmailOTPFailures :exec
DELETE FROM email_otp_failures WHERE email = $1;
