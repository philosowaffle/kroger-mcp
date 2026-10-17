# Spec: Prune stale docs and standards references

**Issue:** #30
**Status:** In Progress

## Problem

Several docs still describe the earlier CLI or generic template defaults, which confuses
contributors. CLAUDE.md describes MySQL, sqlc, `db/schema/` and a `task db:generate` that
doesn't exist; the project has no database and keeps its state in `state.json`. Spec
filenames are inconsistent, and the runbook and architecture docs predate the security
fixes (#34, #37).

## Acceptance Criteria

- [ ] No doc references a database, sqlc or `db:generate`
- [ ] ADR 001 status is Superseded
- [ ] Spec filenames follow the `NNN-short-title.md` convention
- [ ] The runbook reflects the final env vars and defaults

## Notes

- CLAUDE.md's Database, Task Runner and Containerization sections come from the synced
  standards template. Record the deviation in CLAUDE.md so `syncing-standards` flags it
  as a conflict instead of reintroducing the database sections.
- Specs written without an issue are numbered after the PR that landed them.
