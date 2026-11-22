# Spec: SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT and repo security settings

**Issue:** #23
**Status:** In Progress

## Problem

This server holds a shopper's Kroger refresh token and can act on their cart, so people
need a clear, private way to report vulnerabilities. Outside contributors also need to
know the workflow: Conventional Commits, branch naming, spec-first, squash merges.

## Acceptance Criteria

- [x] `SECURITY.md` covers supported versions, private reporting through GitHub Security
      Advisories, scope, and a threat model: the two auth layers, `state.json`
      sensitivity, the unauthenticated callback with single-use state, and running with
      MCP auth off.
- [x] `CONTRIBUTING.md` condenses CLAUDE.md for humans: Task, tests against the
      `krogertest` fake, how to add a tool, PR rules.
- [x] `CODE_OF_CONDUCT.md` exists (defers to GitHub's Community Guidelines; reports go to GitHub).
- [x] `.github/ISSUE_TEMPLATE/config.yml` turns off blank issues and links security
      reports to the advisory form.
- [ ] GitHub's Community Standards page shows every item complete (maintainer check
      after merge).
- [ ] The "Report a vulnerability" button is available (maintainer setting).
- [ ] The release workflow still succeeds with branch protection on (maintainer setting).

## Notes

Repo settings are changed by the maintainer in GitHub, not in this PR:

- Private vulnerability reporting on
- Secret scanning and push protection on
- Dependabot alerts on
- Branch ruleset on `main`: require PRs and PR Checks, block force-pushes
- Squash-only merges, and delete branches on merge

The last three acceptance criteria depend on those settings, so they're checked
after merge, not in this PR.

- The release workflow's changelog job pushes `CHANGELOG.md` straight to `main` with
  `GITHUB_TOKEN`, which a ruleset that requires PRs blocks. Rather than reworking the
  job into a PR, the runbook now says to add the **GitHub Actions** app as a bypass
  actor on the `main` ruleset. The release and image publish before that job, so a
  missing bypass only skips the changelog commit.
- `CODE_OF_CONDUCT.md` started as Contributor Covenant 2.1. Per review, it now defers to
  the GitHub Community Guidelines and Acceptable Use Policies, and violations are reported
  to GitHub (Report content / Block or report) instead of to the maintainer by email. A
  short enforcement section covers what maintainers can do in the repo.
- `SECURITY.md` supports only the latest release and the `main` image while the
  project is pre-1.0.
- The README's Development section links the three new files. The README rewrite
  (#21) will move them into its footer.
