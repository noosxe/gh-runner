-- name: CreateRenovateRun :one
INSERT INTO renovate_runs (
    pool_id,
    status,
    started_at,
    summary,
    container_id
) VALUES (
    ?, ?, ?, ?, ?
) RETURNING *;

-- name: UpdateRenovateRunContainerID :exec
UPDATE renovate_runs
SET container_id = ?
WHERE id = ?;

-- name: CompleteRenovateRun :one
UPDATE renovate_runs
SET status = ?,
    completed_at = ?,
    summary = ?
WHERE id = ?
RETURNING *;

-- name: CompleteRenovateRunByContainerID :one
UPDATE renovate_runs
SET status = ?,
    completed_at = ?,
    summary = ?
WHERE container_id = ?
RETURNING *;

-- name: GetRenovateRun :one
SELECT * FROM renovate_runs
WHERE id = ? LIMIT 1;

-- name: GetRenovateRunByContainerID :one
SELECT * FROM renovate_runs
WHERE container_id = ? LIMIT 1;

-- name: GetLatestRenovateRunByPoolId :one
SELECT * FROM renovate_runs
WHERE pool_id = ?
ORDER BY started_at DESC, id DESC
LIMIT 1;

-- name: ListRenovateRunsByPoolId :many
SELECT * FROM renovate_runs
WHERE pool_id = ?
ORDER BY started_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: CountRenovateRunsByPoolId :one
SELECT COUNT(*) FROM renovate_runs
WHERE pool_id = ?;

-- name: DeleteRenovateRunsByPoolId :exec
DELETE FROM renovate_runs
WHERE pool_id = ?;
