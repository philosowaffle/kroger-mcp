# Spec: Supply-chain hardening for workflows and releases

**Issue:** #28
**Status:** Complete

## Problem

People will run the published image with their Kroger credentials, which makes the build
pipeline part of the security boundary. Actions are pinned to mutable tags, `pr-checks.yml`
inherits the repo's default token scope, golangci-lint floats on `latest`, nothing scans
dependencies, and releases ship without checksums, SBOMs or provenance.

## Acceptance Criteria

- [x] Every `uses:` names a full commit SHA with a `# vX.Y.Z` comment.
- [x] Every workflow sets `permissions: contents: read` at the top and widens it per job.
- [x] golangci-lint is pinned to an exact version.
- [x] `.github/dependabot.yml` covers gomod, github-actions and docker: weekly, grouped.
- [x] govulncheck and CodeQL run on PRs, on pushes to `main` and weekly.
- [x] Release binaries are built with `CGO_ENABLED=0 -trimpath`, and the release ships `SHA256SUMS`.
- [x] Binaries and the image get build-provenance attestations. The image also gets
      an SBOM and `provenance: mode=max`.

## Notes

- Pins stay on each action's current major (checkout v4, setup-go v5,
  golangci-lint-action v8, ...), so this PR changes no behaviour. Dependabot will
  propose the major bumps as its own grouped PR.
- golangci-lint is pinned to v2.14.0, the version used locally.
- govulncheck runs as `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0` in the new
  `security.yml`, not as a step in PR Checks, so the weekly scheduled run shares the
  job. CodeQL uses advanced setup in the same workflow.
- The repo is still private. Attestations (binaries via `subject-checksums: dist/SHA256SUMS`,
  image via `subject-digest` + `push-to-registry`) and CodeQL are gated on
  `!github.event.repository.private`, so they switch on when the repo goes public (#33)
  instead of failing every run now. Verifying a release with
  `gh attestation verify` is therefore deferred to the v0.1.0 release.
- The Dockerfile build also uses `-trimpath`. Base images stay on tags, which
  Dependabot's docker ecosystem watches.
- OpenSSF Scorecard (optional in the issue) is left out. It's most useful once
  the repo is public.
- govulncheck's first run found 34 reachable standard-library vulnerabilities, because
  `setup-go` with `go-version-file` installs exactly the `go` line in `go.mod`
  (`1.26.0`), and release binaries were built with it too. Every `setup-go` now uses
  `go-version: "1.26.x"` with `check-latest: true`, so CI and releases always get the
  newest 1.26 patch. `go.mod` stays at the `1.26.0` minimum. Moving to Go 1.27 means
  changing those `setup-go` lines and the Dockerfile's `golang:1.26-alpine` (Dependabot
  proposes the image bump).
