-- name: CreatePhoneVerification :one
INSERT INTO phone_verifications (user_id, phone_number, amount_xaf, status)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetPhoneVerificationByCampayRef :one
SELECT * FROM phone_verifications WHERE campay_payout_ref = $1;

-- name: GetActivePhoneVerificationByUser :one
SELECT * FROM phone_verifications
WHERE user_id = $1 AND status IN ('initiated', 'pending')
ORDER BY created_at DESC LIMIT 1;

-- name: UpdatePhoneVerificationStatus :one
UPDATE phone_verifications SET
    status = $2,
    failure_reason = $3,
    campay_payout_ref = $4,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: GetLatestPhoneVerificationByUser :one
SELECT * FROM phone_verifications
WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1;