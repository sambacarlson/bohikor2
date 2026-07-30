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
