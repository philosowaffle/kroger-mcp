# CLAUDE.md

@docs/process.md

## Project Overview

kroger-mcp — a self-hosted Go MCP server that wraps the Kroger Public API so Claude can
help with grocery shopping at Kroger-family stores (King Soopers, Kroger, Ralphs, ...).
Tools cover store lookup, product search (price, promo, stock, aisle), linking the
shopper's Kroger account, and adding items to their cart. It serves Streamable HTTP
(not stdio) and ships as a Docker image. Checkout remains manual: the public API is
add-only for carts.

Structure follows go-mcp-template: `internal/*` per concern, `tools/*` per tool group, a
single registry in `internal/server`, and opt-in OAuth protected-resource auth on the MCP
endpoint. See ADR 002.

See `docs/` for full context: `architecture.md` for code structure, `runbook.md` for local
setup, `authentication.md` for both auth layers, `api-research.md` for Kroger API
findings, and `docs/decisions/` for ADRs. The project site lives in `site/` and deploys
to GitHub Pages.

## Project-Specific Notes

- **No database.** Runtime state is a JSON file (`state.json`) in `KROGER_MCP_DATA_DIR`.
- **Kroger API base URL:** `https://api.kroger.com/v1`
- **Two auth layers — keep them separate.** `internal/auth` guards the MCP endpoint
  (`KROGER_MCP_OIDC_*`, delegated to an external IdP). `internal/kroger` holds Kroger
  credentials: an in-memory client-credentials app token, plus the shopper's user token
  from the `kroger_login` flow, persisted in `KROGER_MCP_DATA_DIR/state.json`.
- **Env names.** `server.EnvPrefix` is `KROGER_MCP` (`_ADDR`, `_LOG_LEVEL`, `_OIDC_*`,
  `_DATA_DIR`). The Kroger app settings are `KROGER_*` without the `_MCP` and are read
  only by `internal/kroger`.
- **The Kroger callback route is unauthenticated on purpose** (browser redirect). It
  accepts only single-use states minted by `kroger_login`. Don't move it behind the
  bearer middleware, and don't add ways to mint states outside an MCP tool call.
- **OAuth redirect URI** must match `KROGER_REDIRECT_URI` exactly; its path is the
  callback route (default `http://localhost:8080/kroger/callback`).
- **Runtime state** (`data/`, `.env`) is gitignored and stays local.
- **Dependencies:** go-sdk, go-oidc and httpsnoop only. The Kroger client is standard
  library.
- **Tests:** tool behaviour is tested end-to-end in `internal/server` over the SDK's
  in-memory transport against the fake API in `internal/kroger/krogertest`. Extend the
  fake when adding endpoints rather than mocking the client.
- **Integration tests** live in `integration/` and hit the Kroger certification API
  (`https://api-ce.kroger.com/v1`) with app-token calls only. Cert has no shopper
  accounts, so cart and identity stay out. When you add a read-only tool, add a case to
  `TestMCPTools` too. They run as the `integration` PR check using the
  `KROGER_CERT_CLIENT_ID` / `KROGER_CERT_CLIENT_SECRET` secrets, only after the `checks`
  job passes, to save Kroger API quota.
- **Adding a tool:** add it under `tools/`, register it in `server.New()`, and add a case
  to `internal/server/server_test.go` (including the `TestListTools` list).
- **License:** GPL-3.0-only.
