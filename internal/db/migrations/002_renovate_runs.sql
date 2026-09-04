-- +goose Up
-- +goose StatementBegin

CREATE TABLE renovate_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('running', 'success', 'failure')),
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    summary TEXT NOT NULL DEFAULT '',
    container_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(pool_id) REFERENCES runner_pools(id) ON DELETE CASCADE
);

CREATE INDEX idx_renovate_runs_pool_started ON renovate_runs(pool_id, started_at DESC);
CREATE INDEX idx_renovate_runs_container ON renovate_runs(container_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE renovate_runs;
-- +goose StatementEnd
