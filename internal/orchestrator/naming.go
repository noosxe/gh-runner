package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// Standard metadata labels applied to all supervisor-managed containers (docs/03 §2).
	LabelManaged   = "com.github-runner-supervisor.managed"
	LabelPoolName  = "com.github-runner-supervisor.pool-name"
	LabelID        = "com.github-runner-supervisor.id"
	LabelSpawnedAt = "com.github-runner-supervisor.spawned-at"
	LabelTaskType  = "com.github-runner-supervisor.task-type"
	LabelTargetURL = "com.github-runner-supervisor.target-url"

	// Task types
	TaskTypeRunner = "runner"
	TaskTypeJob    = "task"

	// DefaultRunnerImage is the standard unified runner image.
	DefaultRunnerImage = "ghcr.io/noosxe/runner-aio:latest"

	// DockerMaxContainerNameLen is the maximum container name length enforced by Docker.
	DockerMaxContainerNameLen = 64
)

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateContainerName creates a container name according to OQ #23:
// ghrs-<pool-slug>-<6-hex> with a total length <= 64 characters.
func GenerateContainerName(poolName string) string {
	slug := SlugifyPoolName(poolName)
	randomHex := randomHexSuffix(6)
	return fmt.Sprintf("ghrs-%s-%s", slug, randomHex)
}

// SlugifyPoolName sanitizes and truncates a pool name to fit within the 64-char limit.
// Prefix "ghrs-" is 5 chars, suffix "-xxxxxx" is 7 chars => max slug length is 52 chars.
func SlugifyPoolName(poolName string) string {
	lower := strings.ToLower(strings.TrimSpace(poolName))
	cleaned := nonAlphaNumRegex.ReplaceAllString(lower, "-")
	trimmed := strings.Trim(cleaned, "-")

	if trimmed == "" {
		trimmed = "pool"
	}

	maxSlugLen := DockerMaxContainerNameLen - len("ghrs-") - len("-123456")
	if len(trimmed) > maxSlugLen {
		trimmed = strings.TrimRight(trimmed[:maxSlugLen], "-")
	}
	if trimmed == "" {
		trimmed = "pool"
	}
	return trimmed
}

func randomHexSuffix(length int) string {
	bytesNeeded := (length + 1) / 2
	b := make([]byte, bytesNeeded)
	if _, err := rand.Read(b); err != nil {
		// Fallback timestamp hex if crypto/rand fails
		return fmt.Sprintf("%06x", time.Now().UnixNano()%0xFFFFFF)[:length]
	}
	return hex.EncodeToString(b)[:length]
}
