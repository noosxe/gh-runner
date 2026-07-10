# Open Questions & Clarifications

Before proceeding with the implementation of the AIO Supervisor, the following architectural and product questions need clarification from the product/engineering team:

## 1. Gitea Webhooks & Scale-to-Zero
- Phase 3 proposes Webhook integrations (`workflow_job.queued`) for real-time scaling and scale-to-zero for GitHub. Does Gitea provide an equivalent reliable webhook payload for job queuing that we can ingest? If not, do we rely strictly on the periodic auditor loop for Gitea pools?

## 2. YAML vs. Database Reconciliation
- We mention both YAML import/export and the SQLite DB. We need to define the exact source of truth on boot. If a user mounts a static `config.yml` to the container, does it overwrite the SQLite state? Does it run in a "read-only GitOps mode" where the UI disables editing?

## 3. Environment Variable Specification
- We need to define the formal list of environment variables the Supervisor container itself requires or supports to boot (e.g., `PORT`, `DB_PATH`, `DB_ENCRYPTION_KEY`, `LOG_LEVEL`).

## 4. Data Retention & Pruning Strategy
- We mention a 30-day retention window in the requirements, but we haven't defined the background cron worker or specific query responsible for purging old `job_history` rows and their associated log files to prevent disk exhaustion.

## 5. Error Handling & Rate Limiting
- How should the supervisor handle GitHub/Gitea API rate limits (e.g., backoff strategies) or Docker daemon unreachability without crashing the process or spamming logs?
