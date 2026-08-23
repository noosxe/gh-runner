// Package config implements layered configuration loading (YAML/TOML files,
// environment variables, CLI flags) via koanf, plus validation of the
// supervisor environment contract (DB_ENCRYPTION_KEY, PORT, DB_PATH,
// LOG_LEVEL, DOCKER_HOST, DATA_DIR). Introduced in RUN-7.
package config
