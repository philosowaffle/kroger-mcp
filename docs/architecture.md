# Architecture

## Overview

kroger-mcp is a self-hosted MCP server that exposes the Kroger Public API as
tools, so Claude can find stores, search products and add items to a shopper's
cart. It serves MCP over Streamable HTTP (not stdio) because it is meant to run
in Docker on a home lab and be reached remotely.

It follows the same shape as forge-mcp: one package per concern under
`internal/`, one package per tool group under `tools/`, and a single registry in
`internal/server`. See ADR 002 for why the project pivoted from a CLI.

## Components

```
main.go                     wiring: flags, routes, auth wrap, request log, graceful shutdown
internal/server/            server.New(client): builds mcp.Server, registers every tool
internal/kroger/            Kroger API client: tokens, OAuth callback, locations, products, cart
internal/kroger/krogertest/ in-process fake Kroger API used by tests
internal/auth/              MCP-side OAuth resource server: metadata + bearer JWT validation
internal/health/            /healthz handler + -healthcheck self-probe
internal/logging/           slog setup with a TRACE level and KROGER_MCP_LOG_LEVEL
tools/account/              kroger_login, kroger_auth_status
tools/locations/            search_locations, set_preferred_location
tools/products/             search_products, get_product
tools/cart/                 add_to_cart
```

**`internal/server`** is the tool registry and holds `Name`/`Version` and the
server instructions sent to clients at initialize. To add a tool, create or
extend a package under `tools/` and call it from `server.New()`. That keeps it
covered by the in-memory end-to-end tests in `internal/server/server_test.go`.

**`internal/kroger`** owns everything that talks to Kroger:

- *App token* (client-credentials grant, scope `product.compact`), cached in
  memory and renewed a minute before expiry. Used for products and locations.
- *User token* (authorization-code grant with PKCE; scopes `cart.basic:write
  profile.compact product.compact`). `AuthorizeURL` issues a single-use `state`
  (15 minute TTL) together with a PKCE verifier. `CallbackHandler` exchanges the
  code and persists the tokens. `userAccessToken` refreshes on demand under a
  mutex, so concurrent calls don't spend the same refresh token twice. A
  rejected refresh clears the link, and tools report `ErrNotLinked`.
- *State* (`state.go`): `DATA_DIR/state.json` holds the user token and the
  preferred store. It is written 0600 and swapped in atomically (temp file +
  rename), and memory only changes if the write succeeds.
- API responses are flattened into `Location` and `Product` types that keep only
  what matters for shopping, so tool output stays compact in Claude's context.

**`internal/auth`** makes the MCP endpoint an OAuth protected resource (RFC
9728) when `KROGER_MCP_OIDC_ISSUER` is set. It never runs flows itself. It uses
the SDK's `auth.RequireBearerToken` and `auth.ProtectedResourceMetadataHandler`
and validates RFC 9068 JWTs with go-oidc (discovery + cached JWKS, `aud` must
contain `KROGER_MCP_OIDC_RESOURCE`). The local package is `auth`, so the SDK's
is imported as `mcpauth`.

## HTTP routes

| Route                                  | Auth         | Purpose                                                   |
| -------------------------------------- | ------------ | --------------------------------------------------------- |
| `/healthz`                             | none         | Liveness probe                                            |
| `/.well-known/oauth-protected-resource`| none         | RFC 9728 metadata (only when MCP auth is on)              |
| `/.well-known/*` (anything else)       | none         | Plain 404, so discovery probes aren't derailed by a 401   |
| path of `KROGER_REDIRECT_URI`          | single-use state | Kroger OAuth callback from the shopper's browser     |
| `/`                                    | bearer (if on) | MCP Streamable HTTP                                     |

The Kroger callback sits outside the bearer wrap because a browser redirect
can't carry an MCP token. Only a `state` minted by `kroger_login` is accepted,
and that tool is itself behind MCP auth.

## Data Flow

```
Claude ──MCP/HTTP──► auth middleware ──► mcp.Server ──► tools/* ──► kroger.Client ──HTTPS──► api.kroger.com
                                                                         │
                                                                         └── /data/state.json (user token, preferred store)

Linking:  kroger_login ─► URL(state, PKCE) ─► shopper's browser ─► Kroger login
          ─► redirect to /kroger/callback ─► code exchange ─► state.json
```

Store resolution for product tools: explicit `location_id` → saved preference
(`set_preferred_location`) → `KROGER_LOCATION_ID` → none (national catalog, no
price or stock).

## Key Dependencies

| Dependency                                | Why                                                         |
| ----------------------------------------- | ----------------------------------------------------------- |
| `github.com/modelcontextprotocol/go-sdk`  | Official MCP SDK: server, Streamable HTTP, auth middleware  |
| `github.com/coreos/go-oidc/v3`            | OIDC discovery + JWKS-backed JWT verification for MCP auth  |
| `github.com/felixge/httpsnoop`            | Request-log metrics that keep `http.Flusher` working for SSE |

The Kroger client itself uses only the standard library.

## Testing

- `internal/kroger/krogertest` is a fake Kroger API (token endpoint for all three
  grants, locations, products, cart, profile) that records requests.
- `internal/server` tests drive every tool through a real MCP client session over
  `mcp.NewInMemoryTransports`, backed by the fake.
- `main_test.go` checks routing with MCP auth on and off, including that the
  Kroger callback and well-known paths stay reachable without a token.
- `internal/auth` tests sign real RS256 JWTs and check the middleware against a
  static key set.
