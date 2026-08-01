-- name: CreateCompany :one
INSERT INTO companies (slug, name, created_by)
VALUES ($1, $2, $3) RETURNING *;

-- name: GetCompanyByID :one
SELECT * FROM companies WHERE id = $1 LIMIT 1;

-- name: GetCompanyBySlug :one
SELECT * FROM companies WHERE slug = $1 LIMIT 1;

-- name: ListCompanies :many
SELECT * FROM companies ORDER BY created_at DESC;

-- name: ListCompaniesWithBalance :many
SELECT c.*, COALESCE(SUM(cl.amount_xaf), 0)::NUMERIC(14, 2) AS balance
FROM companies c
LEFT JOIN company_ledger cl ON cl.company_id = c.id
GROUP BY c.id
ORDER BY c.created_at DESC;

-- name: UpdateCompanyStatus :one
UPDATE companies SET status = $2, updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: LockCompanyForFloatCheck :one
-- Acquires a row lock on the company for the life of the caller's
-- transaction, serializing concurrent float-affecting writes (advance
-- debits, reissue debits) against the same company. A balance check made
-- after this call, inside the same transaction, is guaranteed accurate:
-- any concurrent transaction attempting the same lock blocks until this
-- one commits or rolls back, and then observes this transaction's
-- committed ledger writes under READ COMMITTED.
SELECT id FROM companies WHERE id = $1 FOR UPDATE;
