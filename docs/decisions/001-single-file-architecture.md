# 001 — Single-file architecture

**Date:** 2026-03-22
**Status:** Accepted

## Context

kroger-cart is a personal CLI tool with a small, well-defined scope: read a grocery list,
search the Kroger API, and add items to a cart. The question was whether to split the code
into multiple packages (`internal/auth`, `internal/cart`, etc.) or keep everything in a
single `main.go`.

## Decision

All application code lives in `cmd/kroger-cart/main.go`. No `internal/` packages.

## Consequences

**Easier:**
- No navigation between files for a codebase this small
- No circular import concerns
- Trivial builds on any machine — `go build ./cmd/kroger-cart/`
- No supply chain risk — zero external dependencies

**Harder:**
- If the tool grows significantly (multiple subcommands, shared library consumers, testable
  units) the file will need to be split. The code is organized into clearly labeled sections
  to make that refactor straightforward when needed.

The file is organized into named sections (Constants, Data Types, Config, Auth, Location,
Products, Cart, Grocery List, Cache, PKCE Helpers, Utilities) that map cleanly to future
package boundaries if splitting becomes warranted.
