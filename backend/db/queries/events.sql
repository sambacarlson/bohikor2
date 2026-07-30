-- name: CreateEvent :one
INSERT INTO events (company_id, user_id, admin_id, event_type, metadata)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListEventsWithUserByCompany :many
SELECT e.*, u.email AS user_email
FROM events e
LEFT JOIN users u ON e.user_id = u.id
WHERE e.company_id = $1
ORDER BY e.created_at DESC
LIMIT $2 OFFSET $3;
