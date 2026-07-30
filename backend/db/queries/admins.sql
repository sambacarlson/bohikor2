-- name: GetAdminByID :one
SELECT * FROM admins WHERE id = $1 LIMIT 1;

-- name: GetAdminByEmail :one
-- Pre-auth resolution: admin email is globally unique.
SELECT * FROM admins WHERE email = $1 LIMIT 1;

-- name: CreateAdmin :one
INSERT INTO admins (company_id, email, password_hash)
VALUES ($1, $2, $3) RETURNING *;
