# CLAUDE.md

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
- **Versioning:** Semantic Versioning (`vMAJOR.MINOR.PATCH`) — tags are the source of truth
- **Commits:** Conventional Commits format (see below)
- **Branches:** `<type>/issue<#>-short-description`
- **Merge strategy:** Squash and merge only — one commit per issue lands on `main`

## Task Runner

Standard tasks defined in `Taskfile.yml`:

| Task | Description |
| ---- | ----------- |
| `task build` | Build the project binary to `bin/kroger-cart` |
| `task test` | Run all unit tests (`go test ./...`) |
| `task lint` | Run `golangci-lint` |
| `task fmt` | Run `go fmt ./...` |
| `task vet` | Run `go vet ./...` |
| `task docker:build` | Build the Docker image |
| `task docker:up` | Start via docker-compose |
| `task docker:down` | Stop docker-compose |

## Containerization

- `Dockerfile` uses a multi-stage build (golang:1.21-alpine build stage + alpine runtime stage)
- `docker-compose.yml` handles local development — mounts `./data:/data`, exposes port 8080 for OAuth
- Data files (`config.json`, `product_cache.json`, `grocery_list.txt`) live in `./data/` when running via Docker

## Testing

Two categories of tests are expected for every non-trivial feature:

**1. Code path coverage**
General unit tests that exercise the logic — edge cases, error handling, branching behaviour.

**2. Spec coverage**
Each Acceptance Criterion in a spec file should have a corresponding test that directly validates it.
Name these tests clearly so the link to the AC is obvious (e.g. `TestParseGroceryList_SkipsComments`).

Additional conventions:
- Tests live alongside the code they test (`_test.go` files)
- Table-driven tests are preferred
- Integration tests are gated behind a build tag (`//go:build integration`)
- CI runs unit tests on every PR; integration tests run if present

## Versioning

- Semantic Versioning: `vMAJOR.MINOR.PATCH`
- Tags are the source of truth (`git tag v0.1.0`)
- Initial release is `v0.1.0`
- Breaking changes → major bump; new features → minor bump; bug fixes → patch bump

## Documentation Structure

### `docs/architecture.md`

High-level system overview: major components, how they interact, key design decisions.

### `docs/runbook.md`

Local development setup and operational reference. Covers prerequisites, first-run OAuth,
runtime files, common tasks, and troubleshooting.

### `docs/decisions/`

Architecture Decision Records (ADRs). One file per decision, named `NNN-short-title.md`.
Current ADRs:
- `001-single-file-architecture.md`

### `docs/specs/`

Feature and change specifications. Create a spec before starting any non-trivial feature.
Name files after the GitHub issue: `issue-NNN-short-title.md`.

## Workflow

### Starting Work on an Issue

1. Confirm the GitHub Issue exists and has the appropriate label (`feat`, `fix`, `chore`, etc.)
2. Set any `Blocked by` relationships on the issue before starting if applicable
3. Branch from `main`:
   ```
   git checkout -b feat/42-short-description
   ```
4. Write the spec file first: `docs/specs/issue-NNN-short-title.md`
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

Commit at every meaningful checkpoint and push frequently. Example progression:

```
docs: write spec for #NNN
docs: implementation plan for #NNN
feat: scaffold [component] for #NNN
feat: implement [core logic] for #NNN
test: add tests for [component] for #NNN
docs: update runbook for #NNN
```

### Commit Format

```
<type>(optional scope): <description>

[optional body]

[optional footer — e.g. Closes #42]
```

Valid types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `perf`

### Opening a Pull Request

1. Revisit all spec files touched during this branch — update status and notes
2. Delete `tmp/` and commit the deletion
3. PR title must be a valid Conventional Commit message — this becomes the squash commit on `main`
4. Apply the matching label (`feat`, `fix`, `chore`, etc.)
5. Reference the issue in the PR body: `Closes #NNN`
6. Wait for all PR checks to pass before considering the PR ready

### Merging

- Squash and merge only
- Delete the branch after merge
- The linked issue closes automatically via `Closes #NNN`

## Branch Naming

```
<type>/<issue#>-short-description
```

Examples:
- `feat/4-confirm-flag`
- `fix/3-cart-response-code`
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

Set native GitHub `Blocks` / `Blocked by` relationships on issues to track dependencies.

## Project-Specific Notes

- **Kroger API base URL:** `https://api.kroger.com/v1`
- **OAuth redirect URI** must be set to `http://localhost:8080/callback` in your Kroger developer app
- **Runtime files** (`config.json`, `product_cache.json`, `grocery_list.txt`) are gitignored and stay local
- **No external Go dependencies** — standard library only, intentional (see ADR 001)
- **First live run is still needed** — see issues #1–3 for what to validate
