#!/bin/bash
set -euo pipefail

# Ensure Go module cache is writable if it exists, preventing tar extraction permission errors from previous runs
if [ -d "/home/runner/go" ]; then
    echo "Ensuring Go module cache at /home/runner/go is writable..."
    chmod -R +w /home/runner/go || true
fi

# Ensure target configuration variables are present
if [ -z "${GITHUB_REPOSITORY_URL:-}" ]; then
    echo "ERROR: GITHUB_REPOSITORY_URL environment variable is not defined." >&2
    exit 1
fi

if [ -z "${RUNNER_TOKEN:-}" ]; then
    echo "ERROR: RUNNER_TOKEN environment variable is not defined." >&2
    exit 1
fi

# Set default configuration variables if not specified
RUNNER_NAME="${RUNNER_NAME:-$(hostname)}"
RUNNER_WORKDIR="${RUNNER_WORKDIR:-_work}"
RUNNER_LABELS="${RUNNER_LABELS:-self-hosted,linux,arm64}"

echo "======================================================="
echo "Initializing Self-Hosted GitHub Actions Runner"
echo "  Target Repo : ${GITHUB_REPOSITORY_URL}"
echo "  Runner Name : ${RUNNER_NAME}"
echo "  Labels      : ${RUNNER_LABELS}"
echo "  Work Dir    : ${RUNNER_WORKDIR}"
echo "======================================================="

# Graceful cleanup and deregistration handler
cleanup() {
    echo ""
    echo "======================================================="
    echo "Shutting down gracefully. De-registering runner..."
    echo "======================================================="
    if ./config.sh remove --token "${RUNNER_TOKEN}"; then
        echo "Runner successfully de-registered."
    else
        echo "WARNING: Failed to de-register runner. It may need to be removed manually on GitHub." >&2
    fi
    exit 0
}

# Trap termination signals
trap 'cleanup' SIGINT SIGTERM

# Register the runner in unattended mode
echo "Configuring the runner..."
./config.sh \
    --url "${GITHUB_REPOSITORY_URL}" \
    --token "${RUNNER_TOKEN}" \
    --name "${RUNNER_NAME}" \
    --work "${RUNNER_WORKDIR}" \
    --labels "${RUNNER_LABELS}" \
    --unattended \
    --replace

# Start the runner in the background and wait on its process ID
echo "Starting the GitHub Actions Runner agent..."
./run.sh &
RUNNER_PID=$!

# Wait for the runner process to complete (blocks here until runner exits or signal is received)
wait "${RUNNER_PID}"
