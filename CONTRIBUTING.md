# Contributing

Thanks for your interest in kroger-mcp. This is a small, self-hosted project, so
the process is light, but a few conventions keep `main` tidy.

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md). Report
security problems privately, as described in [SECURITY.md](SECURITY.md), not in
an issue.

`docs/process.md` (shared with every repo made from go-mcp-template) and `CLAUDE.md`
are the long-form version of this guide, written for AI coding agents. If they
disagree with this guide, they win.

## Prerequisites

- Go (see the `go` line in `go.mod`; CI uses the newest patch of that minor)
- [Task](https://taskfile.dev) (`go-task`)
- [golangci-lint](https://golangci-lint.run) at the version pinned in
  `.github/workflows/pr-checks.yml`
- Docker, if you want to build the image

You don't need Kroger credentials to build or run the unit tests.

## Getting started

```sh
git clone https://github.com/philosowaffle/kroger-mcp.git
cd kroger-mcp
task ci    # format check, vet, lint, build, unit tests: what PR Checks runs
```

To run the server against the real API, create a `.env` (gitignored) with your Kroger
developer app's `KROGER_CLIENT_ID` and `KROGER_CLIENT_SECRET`, and run `task run`. See
[`docs/runbook.md`](docs/runbook.md).

| Task                    | What it does                                        |
| ----------------------- | --------------------------------------------------- |
| `task build`            | Build `bin/kroger-mcp`                              |
| `task run`              | Run the server (reads `.env`)                       |
| `task test`             | Unit tests                                          |
| `task test:integration` | Integration tests against the Kroger cert API       |
| `task lint`             | golangci-lint                                       |
| `task fmt`              | `go fmt ./...`                                      |
| `task ci`               | Everything PR Checks runs                           |

## Workflow

1. **Start from an issue.** Open one with the feature or bug template, or comment
   on an existing one, before starting non-trivial work. Every issue and PR gets a
   label (`feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`).
2. **Branch from `main`** as `<type>/<issue#>-short-description`, for example
   `feat/42-bulk-search`.
3. **Write a spec first** for non-trivial changes: `docs/specs/NNN-short-title.md`,
   with the problem, testable acceptance criteria and notes. Copy
   `docs/specs/000-template.md`.
4. **Commit often.** Branch commits can be informal; only the PR title lands on
   `main`.
5. **Open a PR** using the template:
   - The title is a [Conventional Commit](https://www.conventionalcommits.org)
     (`feat: ...`, `fix(cart): ...`). It becomes the squash commit and feeds the
     changelog.
   - The body says `Closes #NNN`.
   - The spec's status and checkboxes reflect what was built.
   - PR Checks pass.

PRs are **squash-merged only**, so each issue lands as one commit on `main`.

## Code layout

- `internal/*`: one package per concern (`auth`, `kroger`, `server`, ...).
- `tools/*`: MCP tools, one package per tool group.
- `internal/server`: the single registry, `server.New()`.

There are two auth layers: `internal/auth` guards the MCP endpoint, and
`internal/kroger` holds Kroger credentials. Keep them separate. See
[`docs/architecture.md`](docs/architecture.md) and the ADRs in `docs/decisions/`.

Dependencies are deliberately few: the MCP go-sdk, go-oidc and httpsnoop. The
Kroger client uses only the standard library. Discuss a new dependency in the
issue first.

## Tests

Every non-trivial change needs two kinds of test:

- **Code-path tests**: edge cases, errors and branches in the package itself.
- **Spec tests**: one per acceptance criterion, named so the link is obvious
  (`TestSearchProducts_BrandOnly`).

Conventions:

- Tests live next to the code (`_test.go`). Table-driven tests are preferred.
- Tool behaviour is tested end to end in `internal/server` over the SDK's in-memory
  transport, against the fake Kroger API in `internal/kroger/krogertest`. When you
  call a new endpoint, **extend the fake** rather than mocking the client.
- Integration tests live in `integration/` behind the `integration` build tag and hit
  the Kroger certification API with app-token calls only (cert has no shopper
  accounts, so no cart or identity). They need `KROGER_CERT_CLIENT_ID` and
  `KROGER_CERT_CLIENT_SECRET`.

## Adding a tool

1. Add it under `tools/` (an existing group or a new package), with read-only,
   idempotent and destructive annotations set correctly.
2. Register it in `server.New()` in `internal/server`.
3. Add a case to `internal/server/server_test.go`, and add its name to the
   `TestListTools` list.
4. If it's read-only, add a case to `TestMCPTools` in `integration/`.
5. Document it in the README's tool table.

## Releases

Releases are automatic: merging to `main` releases once PR Checks passes, and
the PR title's Conventional Commit type picks the version (breaking change →
major, `feat` → minor, `fix` or `perf` → patch, other types → no release).
Versions follow [SemVer](https://semver.org), and tags are the source of truth.
See [`docs/runbook.md`](docs/runbook.md#releasing).

## License

kroger-mcp is [GPL-3.0](LICENSE). By contributing, you agree that your
contributions are licensed under the same terms.
