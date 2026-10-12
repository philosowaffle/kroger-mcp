# CLAUDE.md

| Standards Version | Last Synced Date |
| ----------------- | ---------------- |
| 1.1               | 2026-03-22       |

## Project Overview

kroger-mcp — a self-hosted Go MCP server that wraps the Kroger Public API so Claude can
help with grocery shopping at Kroger-family stores (King Soopers, Kroger, Ralphs, ...).
Tools cover store lookup, product search (price, promo, stock, aisle), linking the
shopper's Kroger account, and adding items to their cart. It serves Streamable HTTP
(not stdio) and ships as a Docker image. Checkout remains manual: the public API is
add-only for carts.

Structure and auth mirror forge-mcp (`philosowaffle/forge`, `forge-mcp/`): `internal/*`
per concern, `tools/*` per tool group, a single registry in `internal/server`, and
opt-in OAuth protected-resource auth on the MCP endpoint. See ADR 002.

See `docs/` for full context: `architecture.md` for code structure, `runbook.md` for local
setup, `api-research.md` for Kroger API findings, and `docs/decisions/` for ADRs.

## Documentation

All specs and design documents live in `/docs/specs/`.
Architecture, ADRs, and the local dev runbook live in `/docs/`.

## Conventions

- **Language:** Go
- **Task runner:** Task (`go-task`) — see `Taskfile.yml` for all available tasks
- **Database queries:** sqlc — run `task db:generate` after modifying schema or queries
- **Versioning:** Semantic Versioning (`vMAJOR.MINOR.PATCH`) — tags are the source of truth
- **Commits:** Conventional Commits format (see below)
- **Branches:** `<type>/issue<#>-short-description`
- **Merge strategy:** Squash and merge only — one commit per issue lands on `main`

## Task Runner

Standard tasks defined in `Taskfile.yml`:

| Task                | Description                          |
| ------------------- | ------------------------------------ |
| `task build`        | Build the project binary             |
| `task test`         | Run all unit tests (`go test ./...`) |
| `task lint`         | Run `golangci-lint`                  |
| `task fmt`          | Run `go fmt ./...`                   |
| `task vet`          | Run `go vet ./...`                   |
| `task docker:build` | Build the Docker image               |
| `task docker:up`    | Start via docker-compose             |
| `task docker:down`  | Stop docker-compose                  |
| `task db:generate`  | Run `sqlc generate`                  |

## Containerization

- `Dockerfile` uses a multi-stage build (build stage + minimal runtime stage)
- `docker-compose.yml` handles local development (includes a MySQL service if the project uses a database)
- Self-hosted deployment is the default target

## Database

- **MySQL** is the default database
- **sqlc** is used for type-safe query generation
- Schema files live in `db/schema/`, queries in `db/queries/`, config in `db/sqlc.yaml`
- Migrations are managed explicitly — document the chosen migration tool here when one is added

## Testing

Two categories of tests are expected for every non-trivial feature:

**1. Code path coverage**
General unit tests that exercise the logic of a package — edge cases, error handling, branching behaviour. These ensure the code works correctly at a low level and guard against regressions.

**2. Spec coverage**
Each Acceptance Criterion in a spec file should have a corresponding test that directly validates it. These do not need to be full integration tests — a well-structured unit test is acceptable — but in the ideal case they are integration tests that exercise the full path. Name these tests clearly so the link to the AC is obvious (e.g. `TestParseGroceryList_SkipsComments`).

Additional conventions:
- Tests live alongside the code they test (`_test.go` files)
- Table-driven tests are preferred
- Integration tests are gated behind a build tag (`//go:build integration`) and live in a clearly separated package or directory
- CI runs unit tests on every PR; integration tests run if present

## Versioning

- Semantic Versioning: `vMAJOR.MINOR.PATCH`
- Tags are the source of truth (`git tag v1.0.0`)
- Initial release is `v0.1.0`
- Breaking changes → major bump; new features → minor bump; bug fixes → patch bump

## Documentation Structure

### `docs/architecture.md`

High-level system overview: major components, how they interact, key design decisions. Update as the architecture evolves.

```markdown
# Architecture

## Overview
[What does this system do and how is it structured at a high level?]

## Components
[Describe major components and their responsibilities]

## Data Flow
[How does data move through the system?]

## Key Dependencies
[Notable external dependencies and why they were chosen]
```

### `docs/runbook.md`

Local development setup and operational reference. Should be sufficient for a new contributor to get running from scratch.

```markdown
# Runbook

## Prerequisites
- Go 1.2x+
- Docker + Docker Compose
- Task (`go-task`)
- sqlc

## Local Setup
1. Clone the repo
2. Copy `.env.example` to `.env` and fill in values
3. `task docker:up` — start dependencies
4. `task build` — build the binary
5. `task test` — verify everything works

## Common Tasks
See `Taskfile.yml` for all available tasks.

## Troubleshooting
[Add as needed]
```

### `docs/decisions/`

Architecture Decision Records (ADRs). One file per decision, named `NNN-short-title.md` (e.g. `001-use-sqlc-for-database.md`). Write an ADR for any significant technical choice that future contributors should understand.

```markdown
# NNN — Title

**Date:** YYYY-MM-DD
**Status:** Proposed | Accepted | Deprecated | Superseded

## Context
[What situation or problem prompted this decision?]

## Decision
[What was decided?]

## Consequences
[What are the trade-offs? What becomes easier or harder as a result?]
```

### `docs/specs/`

Feature and change specifications. Create a spec before starting any non-trivial feature. Name files after the GitHub issue: `NNN-short-title.md`.

```markdown
# Spec: [Feature Title]

**Issue:** #NNN
**Status:** Draft | In Progress | Complete

## Problem
[Why does this need to exist? What problem does it solve?]

## Acceptance Criteria
- [ ] [Specific, testable condition]
- [ ] [Specific, testable condition]

## Notes
[Approach, constraints, open questions — add as the work progresses]
```

The spec is complete when all acceptance criteria are checked off and the linked issue is closed. The file stays in the repo as a permanent record.

## Workflow

### Starting Work on an Issue

1. Confirm the GitHub Issue exists and has the appropriate label (`feat`, `fix`, `chore`, etc.)
2. Set any `Blocked by` relationships on the issue before starting if applicable
3. Branch from `main`:
   ```
   git checkout -b feat/42-short-description
   ```
4. Write the spec file first: `docs/specs/NNN-short-title.md`
5. Commit immediately:
   ```
   git commit -m "docs: write spec for #NNN"
   git push -u origin feat/42-short-description
   ```

### Implementation Plan

Before writing implementation code, write a brief plan to `tmp/plan.md`:
- What needs to be built or changed
- Key decisions or trade-offs
- Order of operations

Commit and push:
```
git commit -m "docs: implementation plan for #NNN"
git push
```

The `tmp/` directory is for in-progress working notes only. It must be deleted before a PR
can merge (enforced by CI). Its contents are squashed away and never land on `main`.

### Regular Commits During Development

Commit at every meaningful checkpoint and push frequently — this keeps the branch resumable from any machine. Example progression:

```
docs: write spec for #NNN
docs: implementation plan for #NNN
feat: scaffold [component] for #NNN
feat: implement [core logic] for #NNN
test: add tests for [component] for #NNN
docs: update runbook for #NNN
```

Commits on the feature branch can be informal but descriptive. The squash merge commit to `main` must be a valid Conventional Commit.

### Commit Format

```
<type>(optional scope): <description>

[optional body]

[optional footer — e.g. Closes #42]
```

Valid types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `perf`

### Opening a Pull Request

1. Revisit all spec files touched during this branch — update status and notes to reflect the final implementation
2. Delete `tmp/` and commit the deletion
3. PR title must be a valid Conventional Commit message — this becomes the squash commit on `main`
4. Apply the matching label (`feat`, `fix`, `chore`, etc.)
5. Reference the issue in the PR body: `Closes #NNN`
6. PR checklist must be complete before requesting review
7. After opening, wait for PR checks to complete and verify all checks pass before requesting review or considering the PR ready — investigate and resolve any failures before proceeding

### Merging

- Squash and merge only
- Delete the branch after merge
- The linked issue closes automatically via `Closes #NNN`

## Branch Naming

```
<type>/<issue#>-short-description
```

Examples:
- `feat/42-user-auth`
- `fix/17-nil-pointer-crash`
- `chore/8-update-dependencies`

## GitHub Labels

| Label | Description |
| ----- | ----------- |
| `feat` | New feature or enhancement |
| `fix` | Bug fix |
| `chore` | Maintenance, dependencies, tooling |
| `docs` | Documentation only |
| `refactor` | Code change with no behavior change |
| `test` | Test additions or changes |
| `ci` | CI/CD pipeline changes |
| `breaking` | Introduces a breaking change |

Every issue and PR should have at least one label applied.

## GitHub Issues

Use the appropriate issue template when creating issues:
- **Feature:** `.github/ISSUE_TEMPLATE/feature.md`
- **Bug:** `.github/ISSUE_TEMPLATE/bug.md`

Set native GitHub `Blocks` / `Blocked by` relationships on issues to track dependencies — do not use labels or comments for this.

## Project-Specific Notes

- **Kroger API base URL:** `https://api.kroger.com/v1`
- **Two auth layers — keep them separate.** `internal/auth` guards the MCP endpoint
  (`KROGER_MCP_OIDC_*`, delegated to an external IdP). `internal/kroger` holds Kroger
  credentials: an in-memory client-credentials app token, plus the shopper's user token
  from the `kroger_login` flow, persisted in `DATA_DIR/state.json`.
- **The Kroger callback route is unauthenticated on purpose** (browser redirect). It
  accepts only single-use states minted by `kroger_login`. Don't move it behind the
  bearer middleware, and don't add ways to mint states outside an MCP tool call.
- **OAuth redirect URI** must match `KROGER_REDIRECT_URI` exactly; its path is the callback route
  (default `http://localhost:8080/kroger/callback`).
- **Runtime state** (`data/`, `.env`) is gitignored and stays local.
- **Dependencies:** go-sdk, go-oidc and httpsnoop only. The Kroger client is standard library.
- **Tests:** tool behaviour is tested end-to-end in `internal/server` over the SDK's
  in-memory transport against the fake API in `internal/kroger/krogertest`. Extend the
  fake when adding endpoints rather than mocking the client.
- **Integration tests** live in `integration/` (build tag `integration`) and hit the Kroger
  certification API (`https://api-ce.kroger.com/v1`) with app-token calls only. Cert has no
  shopper accounts, so cart and identity stay out. When you add a read-only tool, add a case
  to `TestMCPTools` too. They run as the `integration` PR check using the
  `KROGER_CERT_CLIENT_ID` / `KROGER_CERT_CLIENT_SECRET` secrets, only after the
  `checks` job passes; `task test:integration` likewise runs vet, lint and unit tests
  first. Both save Kroger API quota.
- **Adding a tool:** add it under `tools/`, register it in `server.New()`, and add a case
  to `internal/server/server_test.go` (including the `TestListTools` list).
- **First live run is still needed**: the ⚠️ items in `docs/api-research.md` are unvalidated.
