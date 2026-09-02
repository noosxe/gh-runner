package provider

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrCredentialLeakage is returned when a master credential appears in a container spec.
	ErrCredentialLeakage = errors.New("credential leakage detected in container spec")
	// ErrForbiddenEnvKey is returned when a forbidden supervisor/db credential environment variable is injected.
	ErrForbiddenEnvKey = errors.New("forbidden environment variable key in container spec")
	// ErrDangerousMount is returned when sensitive database or supervisor files are mounted into a container.
	ErrDangerousMount = errors.New("dangerous host mount detected in container spec")
)

// ContainerSpec models the execution specification of a spawned runner or task container.
type ContainerSpec struct {
	Env    []string          `json:"env"`
	Labels map[string]string `json:"labels"`
	Mounts []string          `json:"mounts"`
	Image  string            `json:"image"`
	Cmd    []string          `json:"cmd"`
}

// SegregationScanner audits container specifications to enforce token segregation (docs/05 §2).
// Master credentials (private keys, supervisor PATs, DB encryption keys) must never be serialized
// into runner container specs, environment variables, labels, or mounts.
type SegregationScanner struct {
	ForbiddenEnvKeys []string
	ForbiddenMounts  []string
}

// NewSegregationScanner creates a new scanner initialized with default segregation rules.
func NewSegregationScanner() *SegregationScanner {
	return &SegregationScanner{
		ForbiddenEnvKeys: []string{
			"DB_ENCRYPTION_KEY",
			"SUPERVISOR_DB_ENCRYPTION_KEY",
			"JWT_SECRET",
			"SUPERVISOR_PAT",
			"GITHUB_APP_PRIVATE_KEY",
			"MASTER_KEY",
			"APP_PRIVATE_KEY",
			"GITEA_MASTER_TOKEN",
			"FORGEJO_MASTER_TOKEN",
		},
		ForbiddenMounts: []string{
			"/data",
			"supervisor.db",
			".db",
			"/etc/supervisor",
		},
	}
}

// Scan examines a container specification against provided master credentials and safety policies.
func (s *SegregationScanner) Scan(spec ContainerSpec, masterCredentials ...string) error {
	// 1. Audit Environment Variables
	for _, env := range spec.Env {
		parts := strings.SplitN(env, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}

		// Check against forbidden keys
		for _, forbiddenKey := range s.ForbiddenEnvKeys {
			if strings.EqualFold(key, forbiddenKey) {
				return fmt.Errorf("%w: prohibited key %q", ErrForbiddenEnvKey, key)
			}
		}

		// Check for master credential leakage in key or value
		for _, secret := range masterCredentials {
			cleanSecret := strings.TrimSpace(secret)
			if len(cleanSecret) < 6 {
				continue
			}
			if strings.Contains(key, cleanSecret) {
				return fmt.Errorf("%w: master credential leaked in environment key %q", ErrCredentialLeakage, key)
			}
			if strings.Contains(val, cleanSecret) {
				return fmt.Errorf("%w: master credential leaked in environment variable %q", ErrCredentialLeakage, key)
			}
		}
	}

	// 2. Audit Labels
	for k, v := range spec.Labels {
		for _, secret := range masterCredentials {
			cleanSecret := strings.TrimSpace(secret)
			if len(cleanSecret) < 6 {
				continue
			}
			if strings.Contains(k, cleanSecret) || strings.Contains(v, cleanSecret) {
				return fmt.Errorf("%w: master credential leaked in container label %q", ErrCredentialLeakage, k)
			}
		}
	}

	// 3. Audit Mounts
	for _, m := range spec.Mounts {
		for _, forbiddenMount := range s.ForbiddenMounts {
			if strings.Contains(m, forbiddenMount) {
				return fmt.Errorf("%w: dangerous path %q in mount %q", ErrDangerousMount, forbiddenMount, m)
			}
		}

		for _, secret := range masterCredentials {
			cleanSecret := strings.TrimSpace(secret)
			if len(cleanSecret) < 6 {
				continue
			}
			if strings.Contains(m, cleanSecret) {
				return fmt.Errorf("%w: master credential leaked in container mount %q", ErrCredentialLeakage, m)
			}
		}
	}

	// 4. Audit Command Args
	for _, arg := range spec.Cmd {
		for _, secret := range masterCredentials {
			cleanSecret := strings.TrimSpace(secret)
			if len(cleanSecret) < 6 {
				continue
			}
			if strings.Contains(arg, cleanSecret) {
				return fmt.Errorf("%w: master credential leaked in command argument", ErrCredentialLeakage)
			}
		}
	}

	return nil
}

// BuildRunnerEnv constructs the standard segregated environment slice for a runner container.
func BuildRunnerEnv(provider string, token string, targetURL string, runnerName string, labels []string, workDir string) []string {
	env := []string{
		"RUNNER_PROVIDER=" + provider,
		"RUNNER_TOKEN=" + token,
		"RUNNER_NAME=" + runnerName,
		"RUNNER_WORKDIR=" + workDir,
		"RUNNER_LABELS=" + strings.Join(labels, ","),
	}

	switch provider {
	case "github":
		env = append(env, "GITHUB_REPOSITORY_URL="+targetURL)
	case "gitea":
		env = append(env, "GITEA_INSTANCE_URL="+targetURL)
	case "forgejo":
		env = append(env, "FORGEJO_INSTANCE_URL="+targetURL)
	}

	return env
}

// BuildRenovateEnv constructs the standard segregated environment slice for a Renovate task container.
func BuildRenovateEnv(provider string, token string, repoURL string) []string {
	return []string{
		"RENOVATE_PLATFORM=" + provider,
		"RENOVATE_TOKEN=" + token,
		"RENOVATE_ENDPOINT=" + repoURL,
	}
}
