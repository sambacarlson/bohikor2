-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token_hash, subject_id, subject_type, expires_at)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW() LIMIT 1;

-- name: RevokeRefreshToken :exec
DELETE FROM refresh_tokens WHERE token_hash = $1;

-- name: RevokeAllRefreshTokensForSubject :exec
DELETE FROM refresh_tokens WHERE subject_id = $1 AND subject_type = $2;

-- name: CleanupExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < NOW();
