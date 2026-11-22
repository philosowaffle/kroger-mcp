# Spec: Rename the repo to kroger-mcp

**Issue:** #32
**Status:** Complete

## Problem

The repo was `philosowaffle/kroger-cart`, but the binary, Docker image
(`ghcr.io/philosowaffle/kroger-mcp`) and docs all say `kroger-mcp`. "kroger-cart" is left
over from the original CLI, and the mismatch is confusing for discovery and installs.

## Acceptance Criteria

- [x] The repo is named `kroger-mcp`
- [x] The module path is updated, and `go build ./...` and `go test ./...` pass
- [x] No references to `kroger-cart` remain outside historical records

## Notes

- The GitHub rename was done in settings; GitHub redirects the old URLs and remotes.
- The module path is now `github.com/philosowaffle/kroger-mcp`: `go.mod`, every import,
  the version `-ldflags` in the Dockerfile, Taskfile and Release workflow, and the
  `goimports` local prefix in `.golangci.yml`.
- The runbook's `gh attestation verify --repo` now names `kroger-mcp`. No release was cut
  under the old name, so there are no attestations signed for `kroger-cart` to verify.
- ADRs 001 and 002 still say "kroger-cart" on purpose: they describe the original CLI as
  it was named at the time, like changelog entries.
- The GHCR image name was already `kroger-mcp` and doesn't change.
