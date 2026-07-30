-- name: CreateAdvanceRequest :one
INSERT INTO advance_requests (company_id, user_id, amount_xaf, status)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetAdvanceRequestByID :one
SELECT * FROM advance_requests WHERE id = $1;

-- name: GetAdvanceRequestByCampayRef :one
-- Webhook resolves by Campay reference (globally unique); no company context.
SELECT * FROM advance_requests WHERE campay_payout_ref = $1;

-- name: GetActiveRequestByUserID :one
SELECT * FROM advance_requests
WHERE user_id = $1 AND status IN ('initiated', 'processing', 'pending')
LIMIT 1;

-- name: ListAdvanceRequestsByUserID :many
SELECT * FROM advance_requests
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListAdvanceRequestsWithUserByCompany :many
SELECT ar.*, u.email AS user_email
FROM advance_requests ar
JOIN users u ON ar.user_id = u.id
WHERE ar.company_id = $1
ORDER BY ar.created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAdvanceRequestStatus :one
UPDATE advance_requests SET
    status = $2,
    failure_reason = $3,
    payout_duration_seconds = $4,
    campay_payout_ref = $5,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: CountAdvanceRequestsByUserToday :one
SELECT COUNT(*) FROM advance_requests
WHERE user_id = $1 AND created_at::date = CURRENT_DATE;

-- name: CountSuccessfulAdvanceRequestsByUserThisMonth :one
SELECT COUNT(*) FROM advance_requests
WHERE user_id = $1 AND status = 'success'
    AND date_trunc('month', created_at) = date_trunc('month', CURRENT_DATE);
