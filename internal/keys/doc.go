// Package keys derives the supervisor's runtime secrets from the single
// master key configured through SUPERVISOR_DB_ENCRYPTION_KEY (RUN-9).
//
// One master key in, two independent secrets out: HKDF (RFC 5869, SHA-256)
// expands the configured master key into the AES-256 database encryption
// key and the JWT signing secret, each under its own context label. The
// labels keep the outputs domain-separated — deriving or leaking one
// secret reveals nothing about the other — while the deterministic
// derivation guarantees that the same configured master key yields the
// same secrets on every boot, so encrypted database rows and issued
// session tokens survive process restarts and binary upgrades. This is
// why there is no separate JWT_SECRET environment variable
// (docs/open-questions.md #3, docs/05-security-and-isolation.md §5).
package keys
