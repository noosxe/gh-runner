-- name: CreateRenovateConfig :one
INSERT INTO renovate_configs (
    pool_id,
    enabled,
    cron_schedule,
    image
) VALUES (
    ?, ?, ?, ?
) RETURNING *;

-- name: GetRenovateConfigByPoolId :one
SELECT * FROM renovate_configs
WHERE pool_id = ? LIMIT 1;

-- name: ListRenovateConfigs :many
SELECT * FROM renovate_configs
ORDER BY pool_id ASC;

-- name: ListEnabledRenovateConfigs :many
SELECT * FROM renovate_configs
WHERE enabled = 1
ORDER BY pool_id ASC;

-- name: UpdateRenovateConfig :one
UPDATE renovate_configs
SET enabled = ?,
    cron_schedule = ?,
    image = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE pool_id = ?
RETURNING *;

-- name: DeleteRenovateConfigByPoolId :exec
DELETE FROM renovate_configs
WHERE pool_id = ?;
