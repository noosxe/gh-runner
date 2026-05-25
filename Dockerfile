# Use standard Ubuntu 22.04 base image to ensure glibc and action tool compatibility
FROM ubuntu:22.04

# Prevent interactive prompts during apt package configuration
ENV DEBIAN_FRONTEND=noninteractive

# Configure Go to leave the module cache writable, preventing "File exists" / permission errors during cache restore on self-hosted runners
ENV GOFLAGS="-modcacherw"

# Install essential CLI tools required for runner setup and basic runner workflows
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    tar \
    sudo \
    ca-certificates \
    git \
    git-lfs \
    jq \
    iproute2 \
    libatomic1 \
    make \
    unzip \
    zip \
    wget \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Establish a dedicated non-root user for security isolation (Least Privilege)
RUN useradd -m -u 1001 runner \
    && usermod -aG sudo runner \
    && echo "runner ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Setup the workspace directory for the runner application
WORKDIR /actions-runner

# External build arguments passed during buildx or single-arch compilation
ARG RUNNER_VERSION=2.334.0
ARG TARGETARCH

# Download and extract the matching actions runner agent tarball
# Maps Docker standard architecture TARGETARCH (amd64 / arm64) to GitHub package architecture tags (x64 / arm64)
RUN set -ex; \
    if [ "${TARGETARCH}" = "amd64" ] || [ -z "${TARGETARCH}" ]; then \
        GH_ARCH="x64"; \
    elif [ "${TARGETARCH}" = "arm64" ]; then \
        GH_ARCH="arm64"; \
    else \
        echo "ERROR: Unsupported Target Architecture: ${TARGETARCH}" >&2; exit 1; \
    fi; \
    curl -o actions-runner.tar.gz -L "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-${GH_ARCH}-${RUNNER_VERSION}.tar.gz"; \
    tar xzf ./actions-runner.tar.gz; \
    rm actions-runner.tar.gz

# Pre-install dotnet runtime and OS dependencies required by the runner agent
RUN ./bin/installdependencies.sh

# Secure workspace directory ownership
RUN chown -R runner:runner /actions-runner

# Copy and secure the custom orchestration entrypoint script
COPY src/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch execution context to the dedicated non-root user
USER runner

# Run the orchestration script
ENTRYPOINT ["/entrypoint.sh"]
