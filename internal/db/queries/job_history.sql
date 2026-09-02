-- name: CreateJobHistory :one
INSERT INTO job_history (
    pool_id,
    runner_name,
    status,
    queued_at,
    started_at,
    completed_at,
    log_retention_path
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetJobHistoryById :one
SELECT * FROM job_history
WHERE id = ? LIMIT 1;

-- name: ListJobHistory :many
SELECT * FROM job_history
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: ListJobHistoryByPoolId :many
SELECT * FROM job_history
WHERE pool_id = ?
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: UpdateJobHistoryStatus :one
UPDATE job_history
SET status = ?,
    completed_at = ?,
    log_retention_path = ?
WHERE id = ?
RETURNING *;

-- name: DeleteJobHistoryOlderThan :exec
DELETE FROM job_history
WHERE completed_at < ?;

-- name: CountJobHistory :one
SELECT COUNT(*) FROM job_history;

-- name: CountJobHistoryByPoolId :one
SELECT COUNT(*) FROM job_history
WHERE pool_id = ?;

-- name: PruneJobHistoryOlderThan :many
DELETE FROM job_history
WHERE (completed_at IS NOT NULL AND completed_at < ?)
   OR (completed_at IS NULL AND created_at < ?)
RETURNING id, log_retention_path;
