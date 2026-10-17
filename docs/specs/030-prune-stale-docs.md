# Spec: Prune stale docs and standards references

**Issue:** #30
**Status:** Complete

## Problem

Several docs still describe the earlier CLI or generic template defaults, which confuses
contributors. CLAUDE.md describes MySQL, sqlc, `db/schema/` and a `task db:generate` that
doesn't exist; the project has no database and keeps its state in `state.json`. Spec
filenames are inconsistent, and the runbook and architecture docs predate the security
fixes (#34, #37).

## Acceptance Criteria

- [x] No doc references a database, sqlc or `db:generate`
- [x] ADR 001 status is Superseded
- [x] Spec filenames follow the `NNN-short-title.md` convention
- [x] The runbook reflects the final env vars and defaults

## Notes

- CLAUDE.md's Database, Task Runner and Containerization sections come from the synced
  standards template. Record the deviation in CLAUDE.md so `syncing-standards` flags it
  as a conflict instead of reintroducing the database sections.
- Specs written without an issue are numbered after the PR that landed them:
  `015-mcp-server-pivot.md`, `018-kroger-api-coverage.md`, `034-integration-tests.md`.
  The feature issue template's `issue-NNN-...` hint now matches.
- ADR 001 was already marked Superseded by 002; no change needed.
- The only remaining mentions of a database, sqlc or `db:generate` are the deviation
  note in CLAUDE.md and this spec, both of which explain their absence.
- Runbook gains the fail-closed listen default, the compose opt-in, and troubleshooting
  for the startup refusal and 429s. Architecture gains the exposure check, rate-limit
  handling and `KROGER_MCP_OIDC_REQUIRE_AT_JWT`, and uses `KROGER_MCP_DATA_DIR`.
