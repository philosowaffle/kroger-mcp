# Spec: SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT and repo security settings

**Issue:** #23
**Status:** In Progress

## Problem

This server holds a shopper's Kroger refresh token and can act on their cart, so people
need a clear, private way to report vulnerabilities. Outside contributors also need to
know the workflow: Conventional Commits, branch naming, spec-first, squash merges.

## Acceptance Criteria

- [ ] `SECURITY.md` covers supported versions, private reporting through GitHub Security
      Advisories, scope, and a threat model: the two auth layers, `state.json`
      sensitivity, the unauthenticated callback with single-use state, and running with
      MCP auth off.
- [ ] `CONTRIBUTING.md` condenses CLAUDE.md for humans: Task, tests against the
      `krogertest` fake, how to add a tool, PR rules.
- [ ] `CODE_OF_CONDUCT.md` is the Contributor Covenant.
- [ ] `.github/ISSUE_TEMPLATE/config.yml` turns off blank issues and links security
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
