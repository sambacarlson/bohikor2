-- name: GetAdminByID :one
SELECT * FROM admins WHERE id = $1 LIMIT 1;

-- name: GetAdminByEmail :one
SELECT * FROM admins WHERE email = $1 LIMIT 1;

-- name: CreateAdmin :one
INSERT INTO admins (email, password_hash)
VALUES ($1, $2) RETURNING *;
