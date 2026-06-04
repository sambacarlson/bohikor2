-- name: ListSettings :many
SELECT * FROM settings ORDER BY key;

-- name: GetSetting :one
SELECT * FROM settings WHERE key = $1;

-- name: UpsertSetting :one
INSERT INTO settings (key, value, updated_by, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW()
RETURNING *;
