// Package config implements the supervisor's layered configuration loading
// (RUN-7): built-in defaults, an optional YAML/TOML file, SUPERVISOR_*
// environment variables, and CLI flags, merged via koanf and validated
// against the environment contract from docs/open-questions.md #3
// (DB_ENCRYPTION_KEY required, PORT, DB_PATH, LOG_LEVEL, DOCKER_HOST,
// DATA_DIR, plus the automated backup schedule).
package config
