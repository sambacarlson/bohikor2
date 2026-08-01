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

-- name: CreateAdvanceRequestReissue :one
INSERT INTO advance_requests (company_id, user_id, amount_xaf, status, reissued_from_id)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListReconcilableAdvanceRequests :many
-- Rows the reconciler must act on: ref'd processing/pending rows past their
-- next_retry_at (a NULL next_retry_at means "never yet scheduled" — treated
-- as due immediately so a row transitioned to processing/pending without
-- explicitly setting next_retry_at, e.g. the post-transfer-DB-update-failure
-- fallback in CreateRequest, is still picked up on the very next tick);
-- processing rows with no campay_payout_ref (pure timeout/crash, nothing to
-- poll — surfaced every tick, self-gated by the reconciler's own grace-period
-- check since polling isn't possible for this class); or initiated rows old
-- enough to be a process-crash artifact (debit committed, server died before
-- the Campay call returned). Excludes rows already flagged for a human.
SELECT * FROM advance_requests
WHERE needs_admin_review = FALSE
  AND (
    (status IN ('processing', 'pending') AND campay_payout_ref IS NOT NULL AND (next_retry_at IS NULL OR next_retry_at <= NOW()))
    OR (status = 'processing' AND campay_payout_ref IS NULL)
    OR (status = 'initiated' AND created_at <= NOW() - INTERVAL '60 seconds')
  )
ORDER BY created_at ASC;

-- name: UpdateAdvanceRequestReconcileAttempt :one
UPDATE advance_requests SET
    attempt_count = attempt_count + 1,
    last_reconciled_at = NOW(),
    next_retry_at = $2,
    needs_admin_review = $3,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: ResolveAdvanceRequest :one
UPDATE advance_requests SET needs_admin_review = FALSE, updated_at = NOW()
WHERE id = $1 AND needs_admin_review = TRUE RETURNING *;
