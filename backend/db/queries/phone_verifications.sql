-- name: CreatePhoneVerification :one
INSERT INTO phone_verifications (company_id, user_id, phone_number, amount_xaf, status)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetPhoneVerificationByCampayRef :one
-- Webhook resolves by Campay reference (globally unique); no company context.
SELECT * FROM phone_verifications WHERE campay_payout_ref = $1;

-- name: GetActivePhoneVerificationByUser :one
-- Age-bounded to 60s to match the frontend's own retry affordance
-- (bohikor/.../account/page.tsx's canRetry lets the user resubmit once a
-- pending/initiated verification is >=60s old) — without this bound, a
-- verification stuck on a lost webhook would 409-block every retry attempt
-- until the reconciler's full backoff ladder (~23min) gives up on it.
-- Older stuck rows are left for the reconciler to keep resolving in the
-- background; the user is just no longer blocked from starting a fresh one.
SELECT * FROM phone_verifications
WHERE user_id = $1 AND status IN ('initiated', 'processing', 'pending')
  AND created_at > NOW() - INTERVAL '60 seconds'
ORDER BY created_at DESC LIMIT 1;

-- name: UpdatePhoneVerificationStatus :one
UPDATE phone_verifications SET
    status = $2,
    failure_reason = $3,
    campay_payout_ref = $4,
    ussd_code = $5,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: GetLatestPhoneVerificationByUser :one
SELECT * FROM phone_verifications
WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: ListReconcilablePhoneVerifications :many
-- Mirrors ListReconcilableAdvanceRequests (see that query's comment for the
-- full rationale): ref'd pending rows past their next_retry_at, or
-- initiated rows old enough to be a process-crash artifact (row created but
-- the Campay collect call, or the follow-up status write, never completed).
-- Phone verification never uses 'processing' (status only ever moves
-- initiated -> pending -> success/failed), so unlike advance_requests there
-- is no "processing with no ref" case to cover here. Excludes rows already
-- flagged for a human.
SELECT * FROM phone_verifications
WHERE needs_admin_review = FALSE
  AND (
    (status = 'pending' AND campay_payout_ref IS NOT NULL AND (next_retry_at IS NULL OR next_retry_at <= NOW()))
    OR (status = 'initiated' AND created_at <= NOW() - INTERVAL '60 seconds')
  )
ORDER BY created_at ASC;

-- name: UpdatePhoneVerificationReconcileAttempt :one
UPDATE phone_verifications SET
    attempt_count = attempt_count + 1,
    last_reconciled_at = NOW(),
    next_retry_at = $2,
    needs_admin_review = $3,
    updated_at = NOW()
WHERE id = $1 RETURNING *;
