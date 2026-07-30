-- name: CreateLedgerEntry :one
INSERT INTO company_ledger (company_id, entry_type, amount_xaf, advance_request_id, created_by, note)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetCompanyBalance :one
SELECT COALESCE(SUM(amount_xaf), 0)::NUMERIC(14, 2) AS balance
FROM company_ledger WHERE company_id = $1;

-- name: ListLedgerByCompany :many
SELECT * FROM company_ledger
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
