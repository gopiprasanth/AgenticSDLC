# AgenticSDLC

AgenticSDLC is a Go-based, Temporal-orchestrated software delivery workflow platform that models SDLC execution as collaborating agents.

## What is AgenticSDLC?

AgenticSDLC is an "agentic software delivery pipeline" where distinct agents handle key phases of delivery and pass artifacts between stages.

Current scaffolded flow:

1. **Product agent**: captures intent and prepares delivery inputs.
2. **Developer agent**: prepares and applies implementation changes.
3. **Security agent**: validates changes and enforces security gates.

The long-term objective is to provide a production-shaped local development environment with:

- Temporal workflow orchestration
- MongoDB-backed run/event/artifact state
- Agent-to-Agent (A2A) task protocol
- MCP-style tool abstractions
- Human-in-the-loop approvals

### Current A2A Integration

A2A task handoff is integrated between agents in the coordinator flow:

- Product -> Developer (`prd_ready`)
- Developer -> Security (`changeset_ready`)
- Security -> Developer on failure (`remediation_required`)
- Developer -> Security after fix (`remediation_ready`)

See `internal/sdlc/workflow.go` for orchestration + A2A dispatch.

See the detailed roadmap in `docs/plan/Agentic SDLC Workflow Development Plan.md`.

## Tech Stack

- **Language:** Go
- **Workflow Orchestration:** Temporal
- **Persistence:** MongoDB
- **Test Utilities:** Testify + Testcontainers
- **Container Runtime:** Docker (for integration/e2e dependency bootstrapping)

## Prerequisites

Install the following locally:

- Go 1.23+ (toolchain currently pinned via `go.mod`)
- Docker (required for integration and e2e tests)
- Git

Optional but useful:

- Temporal CLI/UI for debugging workflows
- MongoDB Compass for inspecting local test/dev data

## How to Install (Recommended Method)

The recommended approach is to use the provided helper assets so your environment matches project expectations.

### Option A (Recommended): Use Docker prerequisites image

1. Build the prerequisites image:

```bash
docker compose -f docker/compose.prerequisites.yml build
```

2. Start an interactive shell with all required tooling pre-installed:

```bash
docker compose -f docker/compose.prerequisites.yml run --rm prerequisites
```

3. Inside the container, verify tools and run tests:

```bash
go version
docker --version
go test ./internal/... ./tests/...
```

### Option B: Native Linux installer (Debian/Ubuntu)

For local host setup without containerized tooling:

```bash
./scripts/install-prerequisites.sh
```

This helper installs/validates:

- Docker Engine + Compose plugin
- Go toolchain
- Git and core build utilities

## How to Run Locally

### 1) Clone and enter repository

```bash
git clone <your-fork-or-repo-url>
cd AgenticSDLC
```

### 2) Install dependencies

```bash
go mod tidy
```

### 3) Run unit tests

```bash
go test ./internal/...
```

### 4) Run integration tests (Docker required)

```bash
go test ./tests/integration -timeout 45s
```

### 5) Run e2e tests (Docker required)

```bash
go test ./tests/e2e -timeout 45s
```

### 6) Run full test suite

```bash
go test ./internal/... ./tests/...
```

## Helper Assets

- `scripts/install-prerequisites.sh` — native Debian/Ubuntu prerequisite installer
- `docker/Dockerfile.prerequisites` — reproducible dev tooling image
- `docker/compose.prerequisites.yml` — one-command shell with prerequisites

## Repository Structure

- `internal/sdlc/` – core coordination logic and runner abstractions
- `internal/api/contracts/` – A2A and MCP request contract handlers
- `tests/integration/` – containerized integration tests
- `tests/e2e/` – end-to-end flow validation
- `docs/plan/` – staged implementation and milestone plan
- `docs/research/` – supporting architecture and research notes

## License

This project is licensed under the **Creative Commons Attribution 4.0 International (CC BY 4.0)** license.

See [LICENSE](./LICENSE) for full text.
