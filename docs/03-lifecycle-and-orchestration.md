# Lifecycle & Orchestration

This document details the dynamic control loops, state management, and orchestration strategies for the AIO Supervisor.

## 1. Dynamic Ephemeral Lifecycle Control

The supervisor operates a continuous control loop to maintain its ephemeral runner pools:

1. **Boot**: Initializes database connections, loads active runner pool configurations, verifies connection to the host container engine, and validates credentials.
2. **Provisioning**: For each defined pool:
   - Spawns the required number of `min_idle_runners` using the configured runner image.
   - Injects the registration token, repository URL, name, and labels as environment variables.
   - Configures the containers to execute exactly one job and self-terminate.
3. **Monitoring**: Periodically checks the health of running containers.
4. **Replacement (Reaping)**:
   - When an ephemeral runner executes a job, it self-terminates, transitioning the container state to `exited`.
   - The supervisor control loop detects the exited container, removes the dead container, and immediately provisions a fresh idle runner container to restore the target pool count.
5. **Deregistration**: Upon receiving a termination signal, the supervisor executes a Graceful Shutdown.

## 2. Container State Sync & Labeling Strategy

To reconcile the running container state on host restarts or daemon crashes without losing pool references, the supervisor tags every container it provisions with metadata labels:

```ini
com.github-runner-supervisor.managed=true
com.github-runner-supervisor.pool-name=<pool-name>
com.github-runner-supervisor.id=<unique-runner-id>
com.github-runner-supervisor.spawned-at=<timestamp>
```

Upon boot, the supervisor queries the host engine filtering for `com.github-runner-supervisor.managed=true` to dynamically rebuild its in-memory tracking state.

## 3. Real-time Container Audit Engine

While a background polling auditor runs periodically (e.g., every 10 seconds), the Docker provider also listens directly to the Docker Event Stream for real-time reaping:

```go
// Listen for container termination events
messages, errs := cli.Events(ctx, types.EventsOptions{})
```

Upon receiving a `"die"` or `"destroy"` event for a container matching the supervisor labels, the supervisor immediately triggers the provisioning of a replacement runner, keeping pool latency low.

## 4. Target Pool Replenisher & Quota Saturation

- **Replenisher**: Compares the count of active, idle runners for each pool against desired targets. If the active count drops below the target, it schedules new idle containers.
- **Saturation Handling**: When the `Total Allowed Runners` limit is reached, the supervisor queues provisioning requests internally until active containers terminate, preventing host resource depletion.
- **Complete Runner Cleanup (Reaping)**: The supervisor deletes the container write layers and any temporary volumes of exited containers.
- **Hung Runner Auto-Termination**: The supervisor monitors run times and force-terminates any container that exceeds the pool's `max_runner_lifetime_seconds`.

## 5. Graceful Shutdown Protocol

Upon receiving a `SIGTERM` or `SIGINT` termination signal, the daemon executes a structured shutdown to protect active workflow runs:

```mermaid
sequenceDiagram
    participant OS as Operating System
    participant SV as Supervisor Engine
    participant GP as Git Provider API
    participant RC as Runner Containers
    
    OS->>SV: SIGTERM / SIGINT
    SV->>SV: Pause pool replenishing loop
    SV->>GP: Deregister & terminate IDLE runners
    SV->>RC: Allow ACTIVE runners to complete single job (up to timeout)
    Note over SV,RC: Periodically checks active count
    RC-->>SV: Container exits (job finished)
    SV->>OS: Exit cleanly
```
