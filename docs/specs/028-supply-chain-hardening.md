# Spec: Supply-chain hardening for workflows and releases

**Issue:** #28
**Status:** In Progress

## Problem

People will run the published image with their Kroger credentials, which makes the build
pipeline part of the security boundary. Actions are pinned to mutable tags, `pr-checks.yml`
inherits the repo's default token scope, golangci-lint floats on `latest`, nothing scans
dependencies, and releases ship without checksums, SBOMs or provenance.

## Acceptance Criteria

- [ ] Every `uses:` names a full commit SHA with a `# vX.Y.Z` comment.
- [ ] Every workflow sets `permissions: contents: read` at the top and widens it per job.
- [ ] golangci-lint is pinned to an exact version.
- [ ] `.github/dependabot.yml` covers gomod, github-actions and docker: weekly, grouped.
- [ ] govulncheck and CodeQL run on PRs, on pushes to `main` and weekly.
- [ ] Release binaries are built with `CGO_ENABLED=0 -trimpath`, and the release ships `SHA256SUMS`.
- [ ] Binaries and the image get build-provenance attestations. The image also gets
      an SBOM and `provenance: mode=max`.

## Notes
