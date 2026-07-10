# Open Questions & Clarifications

Before proceeding with the implementation of the AIO Supervisor, the following architectural and product questions need clarification from the product/engineering team:

## 1. Gitea Webhooks & Scale-to-Zero
- Phase 3 proposes Webhook integrations (`workflow_job.queued`) for real-time scaling and scale-to-zero for GitHub. Does Gitea provide an equivalent reliable webhook payload for job queuing that we can ingest? If not, do we rely strictly on the periodic auditor loop for Gitea pools?
