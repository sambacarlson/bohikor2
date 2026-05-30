-- name: CreatePhoneOTP :one
INSERT INTO phone_otps (phone_number, code, expires_at)
VALUES ($1, $2, $3) RETURNING *;

-- name: GetPhoneOTPByPhoneNumber :one
SELECT * FROM phone_otps WHERE phone_number = $1 AND expires_at > NOW() ORDER BY created_at DESC LIMIT 1;

-- name: DeletePhoneOTP :exec
DELETE FROM phone_otps WHERE phone_number = $1;

-- name: CleanupExpiredPhoneOTPs :exec
DELETE FROM phone_otps WHERE expires_at < NOW();
