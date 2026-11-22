# Spec: Rewrite the README for a public audience

**Issue:** #21
**Status:** Complete

## Problem

The README was accurate but written for the author. A newcomer needs to know within a few
seconds what this is, whether it fits their setup, how to run it quickly, and what the
risks are.

## Acceptance Criteria

- [x] A new user can go from zero to a linked account using only the Quickstart
- [x] The README includes the AI-assistance disclosure
- [x] The BuyMeACoffee link works and `.github/FUNDING.yml` exists
- [x] Badges render, and all links resolve after the repo rename
- [x] The security callout about running with MCP auth off is present

## Notes

- Order: pitch and badges, an example conversation, a five-step Quickstart, a security
  callout, "What it can't do", tools, a short auth summary, configuration, development,
  AI disclosure, support, disclaimer and license.
- The deep auth material (both auth layers, running without auth, Authelia requirements,
  account linking) moved to `docs/authentication.md`. The README keeps an
  "Authentication" section that summarises it, so the compose file's pointer still works.
  Releasing was already covered in `docs/runbook.md`, so the README now points there.
- `.github/FUNDING.yml` already had `buy_me_a_coffee: philosowaffle`; the README adds a
  badge and a Support section linking to https://buymeacoffee.com/philosowaffle.
- The build badge tracks the Release workflow on `main`, because PR Checks only runs on
  pull requests. The release, Go version and GHCR badges are shields.io badges; the
  dynamic ones only render once the repo is public (#33), and the release badge shows
  "no releases" until v0.1.0.
- Every repo link uses `philosowaffle/kroger-mcp`; the rename is done on GitHub, so they
  resolve now.
- The footer links SECURITY.md and CONTRIBUTING.md. The Pages site link is added by the
  Pages issue (#22), since the site doesn't exist yet.
- The "compared to other Kroger MCP servers" note is left to the parity audit (#38).
