-- name: GetUserByID :one
-- Self lookup: id comes from the authenticated claim; caller asserts company match.
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
-- Pre-auth resolution: user email is globally unique. Returns company_id.
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByPhoneNumber :one
SELECT * FROM users WHERE phone_number = $1 LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    company_id, email, email_verified, full_name,
    phone_number, phone_verified, status,
    pin_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: UpdateUserStatus :one
-- Admin action on a target user; scoped to the admin's company.
UPDATE users SET status = $2, updated_at = NOW()
WHERE id = $1 AND company_id = $3 RETURNING *;

-- name: UpdateTermsAcceptance :one
UPDATE users SET
    is_terms_accepted = $2,
    terms_accepted_at = $3,
    terms_version = $4,
    user_ip_at_consent = $5,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: UpdateUserPinHash :one
UPDATE users SET pin_hash = $2, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: IncrementFailedLoginAttempts :one
UPDATE users SET failed_login_attempts = failed_login_attempts + 1, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: ResetLoginAttempts :one
UPDATE users SET failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: LockUserUntil :one
UPDATE users SET locked_until = $2, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: LockUser :one
UPDATE users SET status = 'locked', updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: UnlockUser :one
-- Admin action on a target user; scoped to the admin's company.
UPDATE users SET status = 'active', failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
WHERE id = $1 AND company_id = $2 RETURNING *;

-- name: UpdatePhoneNumber :one
UPDATE users SET phone_number = $2, phone_verified = false, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: SetPhoneVerified :one
UPDATE users SET phone_verified = true, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: ListUsersByCompany :many
SELECT * FROM users
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
