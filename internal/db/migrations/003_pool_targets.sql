-- +goose Up
-- +goose StatementBegin

CREATE TABLE pool_targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id INTEGER NOT NULL,
    target_url TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(pool_id) REFERENCES runner_pools(id) ON DELETE CASCADE,
    UNIQUE(pool_id, target_url)
);

CREATE INDEX idx_pool_targets_pool_id ON pool_targets(pool_id);
CREATE INDEX idx_pool_targets_target_url ON pool_targets(target_url);

-- Backfill legacy single-target runner_pools into pool_targets
INSERT INTO pool_targets (pool_id, target_url)
SELECT id, repository_url FROM runner_pools WHERE repository_url != '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE pool_targets;
-- +goose StatementEnd
