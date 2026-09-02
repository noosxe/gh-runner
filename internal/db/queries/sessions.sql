-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    expires_at
) VALUES (
    ?, ?, ?
) RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE token_hash = ? LIMIT 1;

-- name: ListSessionsByUserId :many
SELECT * FROM sessions
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = ?;

-- name: DeleteSessionsByUserId :exec
DELETE FROM sessions
WHERE user_id = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < ?;
