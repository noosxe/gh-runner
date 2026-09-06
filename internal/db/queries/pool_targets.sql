-- name: AddPoolTarget :one
INSERT INTO pool_targets (
    pool_id,
    target_url
) VALUES (
    ?, ?
) RETURNING *;

-- name: ListPoolTargetsByPoolId :many
SELECT * FROM pool_targets
WHERE pool_id = ?
ORDER BY target_url ASC;

-- name: ListAllPoolTargets :many
SELECT * FROM pool_targets
ORDER BY pool_id ASC, target_url ASC;

-- name: DeletePoolTargetsByPoolId :exec
DELETE FROM pool_targets
WHERE pool_id = ?;

-- name: DeletePoolTarget :exec
DELETE FROM pool_targets
WHERE pool_id = ? AND target_url = ?;

-- name: GetPoolByTargetUrl :one
SELECT rp.* FROM runner_pools rp
JOIN pool_targets pt ON rp.id = pt.pool_id
WHERE pt.target_url = ?
LIMIT 1;

-- name: ListPoolsByTargetUrl :many
SELECT rp.* FROM runner_pools rp
JOIN pool_targets pt ON rp.id = pt.pool_id
WHERE pt.target_url = ?
ORDER BY rp.name ASC;
