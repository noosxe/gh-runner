#!/bin/bash
set -euo pipefail

# Ensure Go module cache is writable if it exists, preventing tar extraction permission errors from previous runs
if [ -d "/home/runner/go" ]; then
	echo "Ensuring Go module cache at /home/runner/go is writable..."
	chmod -R +w /home/runner/go 2>/dev/null || true
fi

# Determine provider mode from environment variables (docs/04 §2)
PROVIDER_MODE="github"
if [ -n "${FORGEJO_INSTANCE_URL:-}" ] || [ "${RUNNER_PROVIDER:-}" = "forgejo" ]; then
	PROVIDER_MODE="forgejo"
elif [ -n "${GITEA_INSTANCE_URL:-}" ] || [ "${RUNNER_PROVIDER:-}" = "gitea" ]; then
	PROVIDER_MODE="gitea"
elif [ -n "${GITHUB_REPOSITORY_URL:-}" ] || [ "${RUNNER_PROVIDER:-}" = "github" ]; then
	PROVIDER_MODE="github"
fi

# Ensure registration token is provided
if [ -z "${RUNNER_TOKEN:-}" ]; then
	echo "ERROR: RUNNER_TOKEN environment variable is not defined." >&2
	exit 1
fi

# Default configuration settings
RUNNER_NAME="${RUNNER_NAME:-$(hostname)}"
RUNNER_WORKDIR="${RUNNER_WORKDIR:-_work}"
RUNNER_LABELS="${RUNNER_LABELS:-self-hosted,linux}"

CONFIG_PATH=""
RUNNER_PID=""

# Graceful cleanup and deregistration handler (docs/04 §3)
cleanup() {
	echo ""
	echo "======================================================="
	echo "Shutting down ${PROVIDER_MODE} runner gracefully..."
	echo "======================================================="
	if [ -n "${RUNNER_PID:-}" ]; then
		kill -TERM "${RUNNER_PID}" 2>/dev/null || true
	fi
	if [ "${PROVIDER_MODE}" = "github" ]; then
		if [ -f "./config.sh" ]; then
			./config.sh remove --token "${RUNNER_TOKEN}" 2>/dev/null || true
		fi
	elif [ "${PROVIDER_MODE}" = "gitea" ]; then
		if [ -n "${CONFIG_PATH:-}" ] && [ -f "${CONFIG_PATH}" ]; then
			act_runner unregister --config "${CONFIG_PATH}" 2>/dev/null || true
		fi
	elif [ "${PROVIDER_MODE}" = "forgejo" ]; then
		if [ -n "${CONFIG_PATH:-}" ] && [ -f "${CONFIG_PATH}" ]; then
			forgejo-runner unregister --config "${CONFIG_PATH}" 2>/dev/null || true
		fi
	fi
	exit 0
}

# Trap termination signals
trap 'cleanup' SIGINT SIGTERM

echo "======================================================="
echo "Initializing Self-Hosted Runner (${PROVIDER_MODE} mode)"
echo "  Runner Name : ${RUNNER_NAME}"
echo "  Labels      : ${RUNNER_LABELS}"
echo "  Work Dir    : ${RUNNER_WORKDIR}"

if [ "${PROVIDER_MODE}" = "github" ]; then
	GITHUB_URL="${GITHUB_REPOSITORY_URL:-${RUNNER_INSTANCE_URL:-}}"
	if [ -z "${GITHUB_URL}" ]; then
		echo "ERROR: GITHUB_REPOSITORY_URL (or RUNNER_INSTANCE_URL) is not defined." >&2
		exit 1
	fi
	echo "  Target URL  : ${GITHUB_URL}"
	echo "======================================================="

	if [ -d "/actions-runner" ]; then
		cd /actions-runner
	fi

	echo "Configuring GitHub Actions runner..."
	./config.sh \
		--url "${GITHUB_URL}" \
		--token "${RUNNER_TOKEN}" \
		--name "${RUNNER_NAME}" \
		--work "${RUNNER_WORKDIR}" \
		--labels "${RUNNER_LABELS}" \
		--unattended \
		--replace \
		--ephemeral

	echo "Starting GitHub Actions runner agent..."
	./run.sh &
	RUNNER_PID=$!
	wait "${RUNNER_PID}"

elif [ "${PROVIDER_MODE}" = "gitea" ]; then
	GITEA_URL="${GITEA_INSTANCE_URL:-${RUNNER_INSTANCE_URL:-}}"
	if [ -z "${GITEA_URL}" ]; then
		echo "ERROR: GITEA_INSTANCE_URL (or RUNNER_INSTANCE_URL) is not defined." >&2
		exit 1
	fi
	echo "  Target URL  : ${GITEA_URL}"
	echo "======================================================="

	export GITEA_RUNNER_EPHEMERAL=1
	mkdir -p "${RUNNER_WORKDIR}"
	CONFIG_PATH="${RUNNER_WORKDIR}/act_config.yaml"

	echo "Generating Gitea act_runner configuration..."
	act_runner generate-config >"${CONFIG_PATH}"

	echo "Registering Gitea act_runner..."
	act_runner register \
		--no-interactive \
		--instance "${GITEA_URL}" \
		--token "${RUNNER_TOKEN}" \
		--name "${RUNNER_NAME}" \
		--labels "${RUNNER_LABELS}" \
		--config "${CONFIG_PATH}"

	echo "Starting Gitea act_runner daemon..."
	act_runner --config "${CONFIG_PATH}" daemon &
	RUNNER_PID=$!
	wait "${RUNNER_PID}"

elif [ "${PROVIDER_MODE}" = "forgejo" ]; then
	FORGEJO_URL="${FORGEJO_INSTANCE_URL:-${RUNNER_INSTANCE_URL:-}}"
	if [ -z "${FORGEJO_URL}" ]; then
		echo "ERROR: FORGEJO_INSTANCE_URL (or RUNNER_INSTANCE_URL) is not defined." >&2
		exit 1
	fi
	echo "  Target URL  : ${FORGEJO_URL}"
	echo "======================================================="

	export FORGEJO_RUNNER_EPHEMERAL=1
	mkdir -p "${RUNNER_WORKDIR}"
	CONFIG_PATH="${RUNNER_WORKDIR}/forgejo_config.yaml"

	echo "Generating Forgejo runner configuration..."
	forgejo-runner generate-config >"${CONFIG_PATH}"

	echo "Registering Forgejo runner..."
	forgejo-runner register \
		--no-interactive \
		--instance "${FORGEJO_URL}" \
		--token "${RUNNER_TOKEN}" \
		--name "${RUNNER_NAME}" \
		--labels "${RUNNER_LABELS}" \
		--config "${CONFIG_PATH}"

	echo "Starting Forgejo runner daemon..."
	forgejo-runner --config "${CONFIG_PATH}" daemon &
	RUNNER_PID=$!
	wait "${RUNNER_PID}"

else
	echo "ERROR: Unknown runner provider mode: ${PROVIDER_MODE}" >&2
	exit 1
fi
