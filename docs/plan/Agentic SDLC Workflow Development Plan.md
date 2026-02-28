# Agentic SDLC Workflow Development Plan (Temporal + Go Modulith)

## Summary
Build a local-first, production-shaped Agentic SDLC platform in Go using Temporal orchestration, full A2A protocol between agents, MCP tool abstraction, and MongoDB memory.  
MVP is a 3-agent chain: **Product -> Developer -> Security**, with real GitHub integration, `gosec` security gating, CLI-based human approval, and end-to-end recovery proof.

## Scope and Delivery Sequence
1. **Milestone 0 (Week 1): Foundation**
- Create modulith skeleton, Temporal worker/client bootstrapping, MongoDB repositories, config system, secrets loading, structured logging, and tracing IDs.
- Stand up local stack via Docker Compose: Temporal, Temporal UI, MongoDB, app service.
- Define shared contracts for A2A Task/AgentCard and MCP tool request/response envelopes.

2. **Milestone 1 (Weeks 2-4): MVP Vertical Slice**
- Implement Product agent with local Markdown PRD generation and revision.
- Implement Developer agent with OpenAI-backed planning plus real GitHub branch/commit/PR operations against a sandbox repository.
- Implement Security agent with `gosec` scan execution, finding normalization, policy decision (pass/fail), and feedback loop to Developer agent.
- Implement full A2A protocol endpoints between agents (even inside modulith runtime).
- Add Temporal workflow orchestration with retry policies, timeout policies, and CLI HITL signal gate.
- Prove resilience: kill worker during execution and confirm workflow resumes correctly without duplicated side effects.
- MVP completion criterion: one successful E2E run + one fail/fix loop + one recovery demonstration.

3. **Milestone 2 (Weeks 5-6): Quality Agent Expansion**
- Add Quality agent for test-plan generation and executable test checks (start with deterministic test templates; browser/E2E depth can be incrementally enabled).
- Insert Quality phase into workflow after Developer fixes and before final approval.

4. **Milestone 3 (Weeks 7-8): Research Agent Expansion**
- Add Research agent with MCP-compatible interface; start with stub/fallback providers and introduce real web research connector iteratively.
- Feed research brief into Product agent as structured context.

5. **Milestone 4 (Weeks 9-10): Deployment Agent Expansion**
- Add Deployment agent contracts and compensating transaction flow (Saga orchestration in Temporal).
- Keep actual infra side effects behind environment gates until staging rollout is enabled.

6. **Milestone 5 (Week 11): Hardening + Single-VM Deployment**
- Package stack for single-VM Docker deployment (post-local target).
- Add RBAC checks + API key auth across control APIs and tool invocation boundaries.
- Finalize runbooks, failure playbooks, and operational dashboards.

## Public APIs / Interfaces / Types

### Control Plane API
- `POST /api/v1/workflows/sdlc/start`
- `GET /api/v1/workflows/{workflowId}`
- `POST /api/v1/workflows/{workflowId}/signal/approve`
- `POST /api/v1/workflows/{workflowId}/signal/reject`

### CLI Surface
- `agenticctl workflow start --project <id> --prompt-file <path>`
- `agenticctl workflow status --id <workflowId>`
- `agenticctl workflow approve --id <workflowId> --stage <stageName>`
- `agenticctl workflow reject --id <workflowId> --stage <stageName> --reason <text>`

### A2A Interfaces (full protocol in MVP)
- `GET /.well-known/agent.json` for each agent.
- `POST /a2a/tasks` to submit task.
- `GET /a2a/tasks/{taskId}` to poll task state.
- Standard task states: `queued`, `running`, `blocked`, `completed`, `failed`.

### Core Contract Types
```go
type SDLCRequest struct {
  ProjectID string
  Goal string
  Constraints []string
  Repo struct {
    Provider string // github
    Owner string
    Name string
    DefaultBranch string
  }
}

type PRDArtifact struct {
  ArtifactID string
  Version int
  Markdown string
  Approved bool
}

type DevChangeSet struct {
  Branch string
  CommitSHAs []string
  PullRequestURL string
  DiffRef string // claim-check ref in MongoDB
}

type SecurityReport struct {
  Tool string // gosec
  Status string // pass|fail
  Findings []SecurityFinding
}
```

### MCP Tool Contracts (MVP)
- Product tools: `generate_prd_markdown`, `revise_prd_markdown`
- Developer tools: `create_branch`, `commit_changes`, `open_pull_request`
- Security tools: `run_gosec_scan`, `summarize_security_findings`

### Persistence Model (MongoDB)
- `workflow_runs`
- `agent_tasks`
- `artifacts` (PRD, diffs, scan reports via claim-check references)
- `audit_events`
- `tool_invocations`

## End-to-End Data Flow (MVP)
1. User starts workflow with project goal + constraints.
2. Product agent creates PRD markdown artifact.
3. Workflow pauses for CLI approval signal.
4. Developer agent generates implementation plan and applies changes to sandbox GitHub repo.
5. Developer agent opens PR and emits `DevChangeSet`.
6. Security agent runs `gosec`; on fail, sends A2A remediation task back to Developer.
7. Loop continues until security pass or retry limit reached.
8. Workflow emits final report with links to PR, findings, and execution trace IDs.

## Testing and Acceptance Criteria

### Required Test Suites
1. Unit tests for workflow decision logic, retry policies, and idempotency guards.
2. Contract tests for A2A endpoints and MCP tool I/O schemas.
3. Integration tests with Dockerized Temporal + MongoDB + mock/openai provider.
4. GitHub sandbox integration tests (branch/commit/PR lifecycle).
5. Security loop tests validating fail -> remediation -> pass path.
6. Recovery tests: worker crash/restart during each major stage.

### MVP Acceptance Scenarios
1. **Happy path:** approved PRD -> PR created -> security pass -> workflow success.
2. **Security remediation path:** initial fail -> developer fix iteration -> pass.
3. **Recovery path:** forced worker termination mid-run -> deterministic resume.
4. **Approval gate path:** workflow blocks until explicit CLI signal.
5. **Auditability path:** complete run trace persisted in MongoDB with correlated IDs.

## Implementation Standards
1. Deterministic workflow code only in Temporal workflow layer; all side effects in activities.
2. Idempotency keys for external calls (GitHub operations, artifact writes).
3. Strict config separation: local/dev/staging/prod profiles.
4. API key auth + RBAC enforced on control and agent endpoints.
5. Structured logs and trace IDs propagated across workflow/activity/agent boundaries.

## Risks and Mitigations
1. **A2A complexity in MVP:** mitigate via shared internal A2A library + contract tests first.
2. **LLM nondeterminism:** keep reasoning inside activities and persist intermediate artifacts.
3. **GitHub side-effect duplication:** enforce idempotent operation records in MongoDB.
4. **Security false positives (`gosec`):** policy tuning file + severity thresholds + suppression audit trail.
5. **Integration sprawl:** keep non-critical external tools stubbed until post-MVP milestones.

## Assumptions and Defaults (Locked)
1. Runtime starts local-first with Docker; next target is single-VM Docker deployment.
2. LLM strategy is provider-pluggable with OpenAI as default provider.
3. MVP repo operations happen in a dedicated sandbox GitHub repository.
4. Product artifacts are local Markdown in MVP (not Google Docs yet).
5. Security gate uses `gosec` first; Sonar integration can be added in a later hardening phase.
6. Human-in-the-loop is CLI signal-based in MVP.
7. Post-MVP expansion order is **Quality -> Research -> Deployment**.
8. Security baseline is **API keys + RBAC** in MVP, with stronger auth hardening later.

## Development Kickoff (Implemented)

### Delivered Bootstrap Scope
- Initialized Go module layout for SDLC coordination primitives and run lifecycle tracking.
- Added `Coordinator` workflow orchestration shell with Product -> Developer -> Security stage progression and security remediation retry loop.
- Added runner abstraction to separate Temporal workflow starts from MongoDB audit persistence.

### Delivered Test Foundation
1. **Unit tests (happy + unhappy)**
   - Happy path end-to-end coordinator execution.
   - Security fail -> remediation -> pass loop.
   - Security fail after retry limit.
   - Temporal start failure and Mongo audit write failure in runner startup flow.
2. **Integration tests with Docker testcontainers**
   - Temporal and MongoDB containers provisioned together.
   - Workflow coordination path executed against containerized dependency bootstrap.
3. **E2E tests with Docker testcontainers**
   - Temporal client connectivity validated against containerized Temporal frontend.
   - MongoDB write path validated with real insert after remediation success path.

### Next Steps
- Replace in-memory workflow store with Mongo-backed repository implementation and claim-check artifact storage.
- Register real Temporal workflows/activities and move side effects behind activity boundaries.
- Add A2A and MCP contract tests and API handlers.


## Next Steps Progress (Continuation)

### 1) Mongo-backed repository + claim-check artifacts
- Added MongoDB-backed workflow repository implementation for `CreateRun`, `UpdateRun`, and `FindRun`.
- Added claim-check style artifact persistence and retrieval (`SaveArtifact`, `LoadArtifact`) against `artifacts` collection.
- Added integration test coverage validating workflow run persistence and artifact round-trip.

### 2) Temporal workflow/activity registration scaffold
- Added first Temporal workflow definition (`agentic.sdlc.workflow`) with deterministic Product -> Developer -> Security sequencing.
- Added security remediation retry loop in workflow path aligned with existing coordinator semantics.
- Added worker registration helper that wires named workflow and activities for incremental replacement of no-op handlers.

### 3) A2A + MCP contract/API bootstrap
- Added A2A task request contract with validation and HTTP handler.
- Added MCP tool request contract with validation and HTTP handler.
- Added contract handler tests for happy and unhappy payload scenarios.
- Implemented coordinator-level A2A task handoffs between Product, Developer, and Security agents for normal and remediation paths.
- Added Developer -> Product back-to-back requirement clarity communication loop before retrying Developer execution.

### Upcoming immediate follow-ups
- Add Mongo indexes and unique constraints (`workflowId`, `artifactId`) plus idempotency keys for external side-effects.
- Replace no-op Temporal activities with real Product/Developer/Security activity implementations and durable audit writes.
- Expand A2A and MCP schema conformance tests with versioned payload fixtures.
