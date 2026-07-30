-- name: ListSettingsByCompany :many
SELECT * FROM settings WHERE company_id = $1 ORDER BY key;

-- name: GetSetting :one
SELECT * FROM settings WHERE company_id = $1 AND key = $2;

-- name: UpsertSetting :one
INSERT INTO settings (company_id, key, value, updated_by, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (company_id, key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW()
RETURNING *;

-- name: SeedDefaultSettings :exec
-- Seeds the per-company default settings on company creation.
INSERT INTO settings (company_id, key, value) VALUES
    ($1, 'kill_switch_enabled', 'false'),
    ($1, 'request_window_start_day', '15'),
    ($1, 'request_window_end_day', '0'),
    ($1, 'daily_request_limit', '0'),
    ($1, 'monthly_request_limit', '1'),
    ($1, 'advance_amount_xaf', '10000');
