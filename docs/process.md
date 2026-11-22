# Development process

Shared by every repo made from go-mcp-template. This file is owned by the template:
change it there, not here (see "Template sync" below). Repo-specific rules go in
CLAUDE.md under Project-Specific Notes, and win where they differ.

## Conventions

- **Language:** Go. **Task runner:** Task (`go-task`); `task --list` shows the tasks, and
  `task ci` runs what PR Checks runs.
- **Commits and PR titles:** Conventional Commits (`feat`, `fix`, `chore`, `docs`,
  `refactor`, `test`, `ci`, `perf`; `!` for breaking). The PR title becomes the squash
  commit on `main` and the changelog line, and a PR check enforces it.
- **Merging:** squash only. Every PR has a label matching its type, and `Closes #N` when it
  has an issue.
- **Versioning:** SemVer tags (`vMAJOR.MINOR.PATCH`) are the source of truth.
  Merging to `main` releases on its own once PR Checks passes on the merged commit:
  a breaking change bumps major, `feat` minor, `fix` or `perf` patch, and other types
  cut no release. Every merge publishes the `main` image either way. So the PR
  title's type decides the release.

## Workflow

1. Work from an issue. For a non-trivial feature, write `docs/specs/NNN-short-title.md`
   first (NNN is the issue number; copy `docs/specs/000-template.md`).
2. A significant technical choice gets an ADR: `docs/decisions/NNN-short-title.md`,
   numbered in sequence (copy `docs/decisions/000-template.md`). Amend or supersede an
   ADR rather than contradicting it silently.
3. Before opening the PR, update the spec's status and notes to match what was built,
   and update `docs/architecture.md` and `docs/runbook.md` if they changed.
4. A PR is ready only when every check is green.

## Code layout

- `main.go` serves Streamable HTTP: `/healthz`, protected-resource metadata when auth
  is on, and the MCP endpoint (bearer auth when on, cross-origin browser requests
  refused). It won't listen beyond loopback without auth unless
  `<PREFIX>_ALLOW_UNAUTHENTICATED=true`.
- `internal/server` is the one tool registry (`server.New()`), and holds `Name` and
  `EnvPrefix`. Every variable the server reads starts with `EnvPrefix`.
- `tools/<group>` holds a group of tools, registered from `server.New()`.
- `internal/auth`, `internal/health` and `internal/logging` are shared with the
  template: change them there.
- **Adding a tool:** add it under `tools/`, register it in `server.New()`, and add it
  to `TestListTools` and `TestMCPTools`.

## Testing

- Unit tests live beside the code (`_test.go`); prefer table-driven tests.
- Each spec acceptance criterion has a test that checks it, named so the link is
  obvious (e.g. `TestParseGroceryList_SkipsComments`).
- Tools are tested end to end in `internal/server` over the SDK's in-memory
  transport, against fakes of what they wrap (a fake API or CLI), not mocks.
- Integration tests use the `integration` build tag and live in `integration/`. They
  run the server over a real Streamable HTTP client against the real dependency, and
  `TestMCPTools` fails for any tool without a case.

## Template sync

Files listed in `.github/template-manifest` come from
[go-mcp-template](https://github.com/philosowaffle/go-mcp-template). `.template-version`
holds the template commit this repo last synced to. Each week the Template sync workflow
compares it with the template and, when the template has moved, files a `template-sync`
issue with the changes and the steps to apply them.
