-- name: GetInvitationByEmail :one
-- Email is globally unique; returns company_id (used by signup to scope the new user).
SELECT * FROM invitations WHERE email = $1 LIMIT 1;

-- name: CreateInvitation :one
INSERT INTO invitations (company_id, email, invited_by, sent_at)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: AcceptInvitation :one
UPDATE invitations SET status = 'accepted', accepted_at = NOW(), updated_at = NOW()
WHERE email = $1 RETURNING *;
