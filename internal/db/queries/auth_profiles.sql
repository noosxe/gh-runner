-- name: CreateAuthProfile :one
INSERT INTO auth_profiles (
    name,
    auth_method,
    app_id,
    private_key_encrypted,
    token_encrypted
) VALUES (
    ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetAuthProfileById :one
SELECT * FROM auth_profiles
WHERE id = ? LIMIT 1;

-- name: GetAuthProfileByName :one
SELECT * FROM auth_profiles
WHERE name = ? LIMIT 1;

-- name: ListAuthProfiles :many
SELECT * FROM auth_profiles
ORDER BY name ASC;

-- name: CountAuthProfiles :one
SELECT COUNT(*) FROM auth_profiles;

-- name: UpdateAuthProfile :one
UPDATE auth_profiles
SET name = ?,
    auth_method = ?,
    app_id = ?,
    private_key_encrypted = ?,
    token_encrypted = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteAuthProfile :exec
DELETE FROM auth_profiles
WHERE id = ?;
