# CLAUDE.md

| Standards Version | Last Synced Date |
| ----------------- | ---------------- |
| 1.1               | 2026-03-22       |

## Project Overview

King Soopers Cart Builder — a Go CLI tool that reads a plain-text grocery list and
automatically adds each item to the user's King Soopers pickup cart via the Kroger Public API.
The user provides a plain-text grocery list; the tool searches for each item, adds it to the
live King Soopers cart (visible immediately on kingsoopers.com), and reports anything it
couldn't find. Checkout and pickup scheduling remain manual.

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
- **OAuth redirect URI** must be set to `http://localhost:8080/callback` in your Kroger developer app
- **Runtime files** (`config.json`, `product_cache.json`, `grocery_list.txt`) are gitignored and stay local
- **No external Go dependencies** — standard library only, intentional (see ADR 001)
- **First live run is still needed** — see issues #1–3 for what to validate
