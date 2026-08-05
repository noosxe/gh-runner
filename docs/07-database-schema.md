# Database Schema Design

This document defines the SQLite database schemas managed via **Goose** migrations and **SQLc**. All schemas are finalized during the design phase.

## `001_initial_schema.sql`

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL, -- Hashed via bcrypt/argon2
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE auth_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    auth_method TEXT NOT NULL CHECK(auth_method IN ('github_app', 'gitea_token', 'forgejo_token', 'pat')),
    app_id INTEGER,
    private_key_encrypted TEXT, -- AES-256 Encrypted
    token_encrypted TEXT,       -- AES-256 Encrypted (For PAT / Gitea / Forgejo)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE runner_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL CHECK(provider IN ('github', 'gitea', 'forgejo')),
    repository_url TEXT NOT NULL,
    auth_profile_id INTEGER NOT NULL,
    min_idle_runners INTEGER NOT NULL DEFAULT 1,
    max_concurrency INTEGER NOT NULL DEFAULT 5,
    labels TEXT NOT NULL, -- Comma-separated list
    runner_image TEXT NOT NULL,
    allow_docker BOOLEAN NOT NULL DEFAULT 0,
    max_runner_lifetime_seconds INTEGER NOT NULL DEFAULT 7200,
    cpu_limit TEXT,
    memory_limit TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(auth_profile_id) REFERENCES auth_profiles(id)
);

CREATE TABLE renovate_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id INTEGER NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT 0,
    cron_schedule TEXT,
    image TEXT NOT NULL DEFAULT 'renovate/renovate:latest',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(pool_id) REFERENCES runner_pools(id) ON DELETE CASCADE
);

CREATE TABLE job_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_id INTEGER NOT NULL,
    runner_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('success', 'failure', 'cancelled', 'timeout')),
    queued_at DATETIME,
    started_at DATETIME,
    completed_at DATETIME,
    log_retention_path TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(pool_id) REFERENCES runner_pools(id) ON DELETE CASCADE
);

CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE app_settings;
DROP TABLE job_history;
DROP TABLE renovate_configs;
DROP TABLE runner_pools;
DROP TABLE auth_profiles;
DROP TABLE admin_users;
-- +goose StatementEnd
```
