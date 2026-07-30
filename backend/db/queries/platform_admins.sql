-- name: GetPlatformAdminByID :one
SELECT * FROM platform_admins WHERE id = $1 LIMIT 1;

-- name: GetPlatformAdminByEmail :one
SELECT * FROM platform_admins WHERE email = $1 LIMIT 1;

-- name: CreatePlatformAdmin :one
INSERT INTO platform_admins (email, password_hash)
VALUES ($1, $2) RETURNING *;
