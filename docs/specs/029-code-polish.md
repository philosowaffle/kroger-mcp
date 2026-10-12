# Spec: Code polish, stricter lint config and version injection

**Issue:** #29
**Status:** Complete

## Problem

Lint runs with golangci-lint's defaults only, so outside contributors get no explicit
bar. `server.Version` is hardcoded to `"0.1.0"` and nothing overrides it, so every build
reports 0.1.0 to MCP clients.

## Acceptance Criteria

- [x] `.golangci.yml` (v2) enables `gosec`, `errorlint`, `bodyclose`, `noctx`, `revive`,
      `misspell`, `unconvert`, `gocritic`, `errname`, `nilerr` and the `gofumpt` and
      `goimports` formatters; `golangci-lint run` passes and CI uses the file.
- [x] `kroger-mcp -version` and MCP `serverInfo.version` report the release tag in
      release builds (Dockerfile, release workflow binaries) and `dev` locally.
- [x] Start-up logs the version and commit.
- [x] `task ci` exists and mirrors PR Checks (fmt check, vet incl. integration tag,
      lint, build, test).
- [x] `APIError` messages returned to Claude can't carry tokens.

## Notes

- `Version` and `Commit` live in `internal/server` as `var`s set with
  `-ldflags -X`. The Taskfile derives them from `git describe --tags --always --dirty`
  and `git rev-parse --short HEAD`; plain `go build` leaves them `dev` / `unknown`.
- The Dockerfile takes `VERSION` and `COMMIT` build args; the release workflow passes
  them to both the image build and the release binaries.
- Lint findings fixed: `%v` → `%w` when wrapping errors (auth, refresh), the health probe
  now builds its request with a context, and a test's `cancelled` became `canceled`. Two
  are justified with `//nolint`: G304 on reading `state.json` (operator-configured path) and
  a test map key whose whitespace is deliberate. `run.build-tags: [integration]` lints the
  integration tests too.
- `APIError` now redacts `access_token`, `refresh_token`, `id_token`, `client_secret`,
  `code_verifier` and `code` JSON fields, and cuts the body at 500 runes (not bytes, which
  could split a UTF-8 character). Covered by `TestAPIErrorMessage_RedactsSecrets`.
- `main` images report version `main`; `TestServerInfoVersion` checks `serverInfo`.
- Tool descriptions were reviewed and read consistently; only `kroger_login` changed (#25).
