-- name: ListInvitationsByCompany :many
SELECT * FROM invitations WHERE company_id = $1 ORDER BY sent_at DESC;
