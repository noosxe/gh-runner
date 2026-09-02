-- name: GetAppSetting :one
SELECT key, value, updated_at
FROM app_settings
WHERE key = ? LIMIT 1;

-- name: SetAppSetting :one
INSERT INTO app_settings (
    key,
    value,
    updated_at
) VALUES (
    ?,
    ?,
    CURRENT_TIMESTAMP
) ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: ListAppSettings :many
SELECT key, value, updated_at
FROM app_settings
ORDER BY key ASC;

-- name: DeleteAppSetting :exec
DELETE FROM app_settings
WHERE key = ?;
