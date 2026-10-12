# Spec: Code polish, stricter lint config and version injection

**Issue:** #29
**Status:** In Progress

## Problem

Lint runs with golangci-lint's defaults only, so outside contributors get no explicit
bar. `server.Version` is hardcoded to `"0.1.0"` and nothing overrides it, so every build
reports 0.1.0 to MCP clients.

## Acceptance Criteria

- [ ] `.golangci.yml` (v2) enables `gosec`, `errorlint`, `bodyclose`, `noctx`, `revive`,
      `misspell`, `unconvert`, `gocritic`, `errname`, `nilerr` and the `gofumpt` and
      `goimports` formatters; `golangci-lint run` passes and CI uses the file.
- [ ] `kroger-mcp -version` and MCP `serverInfo.version` report the release tag in
      release builds (Dockerfile, release workflow binaries) and `dev` locally.
- [ ] Start-up logs the version and commit.
- [ ] `task ci` exists and mirrors PR Checks (fmt check, vet incl. integration tag,
      lint, build, test).
- [ ] `APIError` messages returned to Claude can't carry tokens.

## Notes

- `Version` and `Commit` live in `internal/server` as `var`s set with
  `-ldflags -X`. The Taskfile derives them from `git describe --tags --always --dirty`
  and `git rev-parse --short HEAD`; plain `go build` leaves them `dev` / `unknown`.
- The Dockerfile takes `VERSION` and `COMMIT` build args; the release workflow passes
  them to both the image build and the release binaries.
