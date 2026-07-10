# Product Requirements: GitHub & Gitea Actions Runner AIO Supervisor

## 1. Executive Summary & Goals

### The Problem
Traditional self-hosted runner setups involve running persistent runner processes. This poses major security and maintenance challenges:
1. **State Persistence**: Successive jobs run in the same container, leading to directory clutter, leftover processes, and potential cross-job data leakage.
2. **Dashboard Clutter**: Static runners that go offline are marked as "offline ghost runners" on GitHub/Gitea, cluttering repository and organization settings.
3. **Scaling Limitations**: Orchestrating runners across multiple repositories or scaling runner capacity dynamically requires complex custom scripting or heavy-weight orchestration tools like Kubernetes (e.g., Actions Runner Controller).

### The Solution: AIO Supervisor
The **AIO Supervisor** is a containerized daemon and web control plane. It manages dynamic, on-demand pools of ephemeral runners by auditing container processes and responding directly to repository platform events.

Key capabilities:
- **Containerized Daemon Deployment**: Runs inside its own dedicated container alongside a local database, communicating with the host container engine via its socket interface.
- **Maintains Dynamic Ephemeral Pools**: Configured to run ephemeral containers, ensuring each runner container executes **exactly one job** and self-destructs immediately.
- **Multi-Repository & Multi-Provider Support**: Simultaneously manages independent runner pools for different GitHub and Gitea repositories and organizations from a single host.
- **Managed Renovate Bot Integrations**: Optionally schedules and orchestrates ephemeral `renovate/renovate` containers via cron to automatically maintain repository dependencies without requiring external CI workflows.
- **Local Admin & Setup Flow**: Requires the user to set up a local administrator account securely on first UI launch. Connecting GitHub and Gitea accounts happens separately from the admin pages.
- **Web Control Interface**: Serves a secure web UI to monitor pool states, search execution history, check success/failure statistics, analyze queue wait-time latency, and view real-time logs.
- **Graceful Lifecycles**: Monitors runner lifetimes, dynamically obtains fresh registration tokens from GitHub/Gitea APIs, replaces terminated containers, and cleanly de-registers them during supervisor shutdown.
- **Secure-by-Default Isolation**: Enforces CPU/Memory constraints, runs under non-root contexts, isolates credentials, and restricts host Docker socket access.

## 2. Functional Requirements

### 2.1 Interactive Onboarding & Setup Flow
For new installations or initial configurations, the supervisor serves a guided, multi-step onboarding setup flow:

- **Step 1: Local Admin Setup (First Launch Only)**:
  - The UI prompts the user to create a local administrator account (username/password). These credentials are securely hashed and stored in the local SQLite database.
- **Step 2: Connect Git Providers & Choose Repositories**:
  - The admin links their GitHub App or Gitea PAT.
  - The UI lists all available Organizations and Repositories.
  - The user checks the specific repositories or organizations they want to onboard for dynamic runner pooling.
- **Step 3: Choose Global Scaling Constraints**:
  - Configures global runner thresholds, including:
    - **Total Allowed Runners**: Absolute maximum number of runner containers executing concurrently across all pools to prevent resource exhaustion.
    - **Total Idle Warm Pool**: The global default count of idle runner containers kept running in a warm state to pick up queued jobs instantly.
- **Step 4: Define Custom Per-Repo / Per-Org Constraints (Optional)**:
  - Users can optionally override global settings on a granular level:
    - Specific pool sizes (`min_idle_runners`, `max_concurrency`) per repository.
    - Custom runner labels.
    - CPU and Memory hard limits per repository pool.
    - Enable Managed Renovate Bot for the repository and configure its cron schedule.
- **Step 5: Review & Confirmation**:
  - Summarizes the planned configuration pools, expected system footprints, and credential setups.
  - Upon user confirmation, the supervisor starts the control loops and dynamic pool provisioning instantly.

### 2.2 Embedded Administration & Web Dashboard
Following configuration, users are redirected to their persistent Web Dashboard containing advanced monitoring:

- **Active Dynamic Pools**:
  - Visual status of each runner pool, listing active container instances, CPU/Memory resource consumption, uptime, and runner name.
- **Job Run Analytics & History**:
  - **Recent Run List**: Displays details of recently executed Actions jobs, with full search, filtering, and pagination capabilities.
  - **Streaming Logs**: Live logs of individual active runners.
  - **Data Retention**: The supervisor enforces a configurable metrics and history retention window (default: 30 days) to automatically prune older logs and database records.
- **Success & Failure Stats**:
  - Displays health and performance aggregates of jobs (success/failure ratio, counts, runtimes).
- **Queue Wait Time Analytics**:
  - *Calculation*: The supervisor hooks into webhook events (e.g., `workflow_job.queued` and `workflow_job.started`).
  - *Formula*: It calculates the queue latency as `started_at` - `queued_at`.
  - *Visual Output*: Graphs the average queue wait time over hours/days, indicating system capacity health and whether additional warm idle runners should be provisioned to decrease latency.

### 2.3 Periodic Runner Image Updates
To keep runner environments secure and up-to-date:
- **Update Checks**: The system periodically checks for new versions of the configured runner images and notifies the admin inside the Web UI when updates are available.
- **Automatic Background Updates**: The admin can configure a schedule for automatic image updates. 
- **Graceful Handoff**: Image updates happen gracefully without disrupting running workflows. Active runners are allowed to finish their current jobs, while all newly provisioned runners automatically spawn using the newly pulled image.

## 3. Development Phases

```text
Phase 1: Design (Current)  -->  Phase 2: Supervisor Daemon & Web UI  -->  Phase 3: Webhook Integrations
• Requirements & Schema         • Static pool scaling                • Real-time webhook scaling
• Threat & Security review      • Embedded Web UI & Dashboard        • Dynamic scale-to-zero
                                • App & PAT auth engine
```

### Phase 2: Core Supervisor Daemon & Web UI
Implement the supervisor as a native, lightweight application capable of:
1. Initializing from database configurations and supporting YAML import/export.
2. Obtaining registration tokens dynamically using App ID or PAT.
3. Interacting with the container engine API to start, stop, and clean up ephemeral containers.
4. Maintaining stable pools of dynamic runners and scheduling Renovate Bot containers.
5. Serving the embedded Web Control Interface to allow visual administration, logging, and metrics.

### Phase 3: Webhook & Autoscaling (Optional Extension)
Extend the supervisor with an HTTP receiver for Webhooks (`workflow_job.queued` and `workflow_job.completed`). This allows scaling the pool down to zero when no jobs are active, and dynamically spinning up runners specifically to match queued jobs.
