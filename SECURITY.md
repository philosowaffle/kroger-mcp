# Security Policy

kroger-mcp holds a shopper's Kroger refresh token and can add items to their cart,
so security reports are taken seriously. Thanks for helping keep it safe.

## Supported versions

kroger-mcp is pre-1.0. Only the latest release, and the `main` image built from the
tip of `main`, get security fixes. Please check that a problem reproduces on one of
them before reporting.

| Version            | Supported |
| ------------------ | :-------: |
| Latest `v0.x.y`    | ✓         |
| `main` image       | ✓         |
| Older releases     |           |

## Reporting a vulnerability

**Don't open a public issue, discussion or pull request for a vulnerability.**

Report it privately through GitHub's
[security advisory form](https://github.com/philosowaffle/kroger-mcp/security/advisories/new)
(**Security → Report a vulnerability**). Please include:

- what an attacker can do, and what they need first (network access, a valid MCP
  token, a login URL, ...)
- steps to reproduce, ideally against a local build
- the version or commit, and relevant config (auth on or off, listen address,
  reverse proxy)

This is a one-person hobby project, so there's no formal SLA. Expect an
acknowledgement within a week. Once a fix is released, the advisory is published
with credit to the reporter unless you'd rather stay anonymous.

Don't include real Kroger tokens, client secrets or `state.json` contents in a
report. Redact them.

## Scope

In scope:

- this repository's Go code, `Dockerfile`, `docker-compose.yml` and GitHub workflows
- the published image (`ghcr.io/philosowaffle/kroger-mcp`) and release binaries

Out of scope:

- the Kroger API itself, and Kroger's own sites and apps. Report those to Kroger.
- your identity provider (Authelia or other) and its configuration
- deployments that ignore the guidance below, such as exposing the server to the
  internet with `KROGER_MCP_ALLOW_UNAUTHENTICATED=true`
- denial of service by spending your own Kroger API quota

## Threat model

### Two auth layers

kroger-mcp has two separate auth layers, and a weakness in either one matters.

1. **Who can use the MCP server** (client → kroger-mcp). Optional OAuth
   protected-resource auth, delegated to your own identity provider. The server never
   issues tokens. It validates a bearer JWT on every request: signature via the
   issuer's JWKS, `iss`, `aud`, `exp`, any required scopes, and by default a
   `typ` of `at+jwt` so an ID token can't stand in for an access token.
2. **Whose Kroger account it acts on** (kroger-mcp → Kroger). An in-memory
   client-credentials app token for product and store lookups, plus the shopper's
   user token from the `kroger_login` flow, which the cart needs.

Anyone who passes layer 1 can act as the linked shopper in layer 2: add items to
their cart, or relink the server to a different Kroger account.

### `state.json` is a credential

`$KROGER_MCP_DATA_DIR/state.json` (`/data/state.json` in the image) holds the
shopper's access and refresh tokens. A refresh token lasts about six months.
The server writes the file with mode `0600` inside a `0700` directory and replaces
it atomically. Treat it, and any backup of it, like a password: anyone with the file
can call the Kroger Cart and Identity APIs as that shopper until the link expires.
Delete it to unlink.

The Kroger client secret comes from the environment (`KROGER_CLIENT_SECRET`) and is
never written to disk by the server. Kroger API error bodies are scrubbed of
token-shaped fields before they reach logs or tool results.

### The unauthenticated callback

`/kroger/callback` is reachable without MCP auth, because a browser redirect can't
carry a bearer token. To keep that safe it only accepts a `state` value that:

- was minted by a `kroger_login` tool call, which does pass layer 1
- is used once, then discarded
- is less than 15 minutes old

The authorization-code exchange also uses PKCE. So nobody can link an account through
the callback without first getting a login URL through an authenticated MCP call.
Any change that lets a state be minted some other way, or moves the callback behind
the bearer middleware, is a security-relevant change.

### Running with MCP auth off

With `KROGER_MCP_OIDC_ISSUER` unset, **anyone who can reach the server can use the
linked Kroger account**, including calling `kroger_login` and linking their own
account in its place so your later cart adds land in their cart. To make that hard
to do by accident:

- With auth off, the server refuses to listen on a non-loopback address unless
  `KROGER_MCP_ALLOW_UNAUTHENTICATED=true` is set.
- The sample `docker-compose.yml` publishes the port on `127.0.0.1` only.
- `kroger_login` warns when an account is already linked, and replacing a link is
  logged with both profile IDs.

Only run without auth on loopback or a network you fully trust (a home LAN, a VPN,
or behind a reverse proxy that enforces its own auth).

### What the Kroger API limits

The public Cart API can only add items. It can't read the cart, remove items or
check out, so the worst a compromised server can do to a shopper's cart is add
unwanted items. It can also read the shopper's Kroger profile ID.

## Hardening checklist for self-hosters

- Turn on MCP auth (`KROGER_MCP_OIDC_*`) for anything beyond localhost.
- Serve the server over HTTPS (a reverse proxy) when it leaves the host.
- Keep `/data` on a volume only the server can read, and keep backups of
  `state.json` private.
- Pin an image tag, verify it (see `docs/runbook.md`), and update when a release
  ships a security fix.
