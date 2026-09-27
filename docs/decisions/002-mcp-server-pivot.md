# 002 — Pivot to a dockerized MCP server

**Date:** 2026-09-27
**Status:** Accepted

## Context

kroger-cart started as a CLI that read `grocery_list.txt`, matched each line to
the first product search result, and added it to the cart. The hard part of
grocery shopping is judgment: picking the right product, brand and size, and
comparing prices or promos. A fixed pipeline does that badly, and Claude does
it well if it has direct access to the Kroger API.

The owner already self-hosts forge-mcp, a Go MCP server that serves Streamable
HTTP and protects its endpoint by delegating OAuth to an external authorization
server (Authelia). Reusing that shape keeps deployment and auth consistent
across their MCP servers.

## Decision

- Replace the CLI with an MCP server (`kroger-mcp`) exposing Kroger stores,
  products, cart-add and account linking as tools.
- Serve Streamable HTTP and ship as a distroless Docker image with a
  self-probing healthcheck, as forge-mcp does.
- Protect the MCP endpoint the way forge-mcp does: opt-in OAuth protected
  resource (RFC 9728), JWT validation via go-oidc, configured through
  `KROGER_MCP_OIDC_*`.
- Handle Kroger's user OAuth inside the server. A `kroger_login` tool issues
  a single-use, PKCE-backed login URL, an unauthenticated callback route
  completes it, and tokens persist to a data volume.
- Split into packages by concern (`internal/*`, `tools/*`), which supersedes
  ADR 001's single-file layout.
- Allow external Go dependencies: the official MCP SDK, go-oidc and httpsnoop.
  The Kroger client itself stays standard library only.
- Drop the grocery-list file and the product cache. Claude does the matching,
  and preferences belong in the conversation or the user's own notes.

## Consequences

**Easier:**
- Claude can compare products, check prices and stock, and ask before adding.
- Same deployment and auth model as forge-mcp.
- Every tool is covered by in-memory MCP end-to-end tests against a fake
  Kroger API.

**Harder:**
- Third-party dependencies need updating.
- The Kroger OAuth callback has to be reachable from the shopper's browser, and
  its redirect URI has to match the Kroger app registration exactly.
- Pending login states live in memory, so a restart invalidates outstanding
  login URLs (acceptable: they're short-lived and cheap to reissue).
- The public API is still add-only for carts. Reading and removing cart items
  would need Kroger's partner API.
