# kroger-mcp

An [MCP](https://modelcontextprotocol.io) server that wraps the
[Kroger Public API](https://developer.kroger.com), so Claude can help you shop at
Kroger-family stores (King Soopers, Kroger, Ralphs, Fred Meyer, Fry's, Smith's,
and others). Written in Go on the official
[`go-sdk`](https://github.com/modelcontextprotocol/go-sdk), it serves
**Streamable HTTP** so you can self-host it in Docker and reach it remotely.

## Tools

| Tool                     | Needs linked account | What it does                                                        |
| ------------------------ | :------------------: | ------------------------------------------------------------------- |
| `search_locations`       |                      | Find stores near a zip code.                                        |
| `set_preferred_location` |                      | Save a default store. It persists across restarts.                  |
| `search_products`        |                      | Search the catalog. Scoped to a store, it adds price, promo, stock, aisle and fulfillment. |
| `get_product`            |                      | Get one product by product id or UPC.                               |
| `kroger_login`           |                      | Get a single-use URL to link your Kroger shopping account.          |
| `kroger_auth_status`     |                      | Check whether an account is linked and which store is the default.  |
| `add_to_cart`            |          ✓           | Add items to your cart by UPC (PICKUP or DELIVERY).                 |

The Kroger public API can **add** to a cart, but it cannot read the cart, remove
items or check out. You finish on the store's website or app.

## Two kinds of auth

There are two separate auth layers, and it helps to keep them apart:

1. **Who can use this MCP server** (Claude → kroger-mcp). This is optional OAuth
   protection of the MCP endpoint, delegated to your own identity provider. It
   uses the same setup as forge-mcp and is described [below](#authentication).
2. **Whose Kroger account it acts on** (kroger-mcp → Kroger). This uses Kroger's
   own OAuth. Product and store lookups use an app token from your Kroger
   developer credentials. The cart needs your shopping account, which you link
   once in a browser (see [Linking your Kroger account](#linking-your-kroger-account)).

## Setup

### 1. Create a Kroger developer app

1. Sign up at <https://developer.kroger.com>. This developer account is separate
   from your shopping account.
2. **My Apps → Create App**. Give it the **Product**, **Cart**, **Profile** and
   **Locations** APIs.
3. Add a **Redirect URI**: the public URL of this server plus `/kroger/callback`.
   For example `https://kroger-mcp.example.com/kroger/callback`, or
   `http://localhost:8080/kroger/callback` for a local run. It has to match
   `KROGER_REDIRECT_URI` exactly.
4. Copy the **Client ID** and **Client Secret**.

### 2. Run it

```sh
export KROGER_CLIENT_ID=...
export KROGER_CLIENT_SECRET=...
export KROGER_REDIRECT_URI=http://localhost:8080/kroger/callback

docker compose up -d     # pulls ghcr.io/philosowaffle/kroger-mcp:latest
```

Compose also reads these from a `.env` file next to `docker-compose.yml`.
State lives in the `kroger-data` volume, mounted at `/data`. To run from
source instead, uncomment `build: .` in `docker-compose.yml` and use
`docker compose up -d --build`, or run `go run .`.

Images are published for `linux/amd64` and `linux/arm64`. Releases are tagged
`latest`, `X.Y.Z` and `X.Y`; the tip of `main` is tagged `main`.

### 3. Connect Claude

```sh
claude mcp add --transport http kroger http://<host>:8080
```

For claude.ai or the desktop app, add a custom connector pointing at the same URL.

### 4. Link your Kroger account

Ask Claude something like "link my Kroger account". It calls `kroger_login` and
gives you a URL. Open it, sign in with your **shopping** account and approve.
Kroger redirects to `/kroger/callback`, and the server stores the tokens in
`/data/state.json` and refreshes them automatically. Links last about six
months; after that, `add_to_cart` asks you to link again.

The callback is reachable without MCP auth, because a browser redirect can't
carry a bearer token. It only accepts the single-use `state` from a
`kroger_login` URL issued in the last 15 minutes, so nobody can use it to link
an account without first getting a login URL through an authenticated MCP call.

Then ask Claude to find your store ("find the King Soopers near 80218 and make
it my default"), and you're ready to shop.

## Authentication

Authentication is off by default. The server runs unauthenticated, which suits
internal, LAN or VPN-only deployments. Set `KROGER_MCP_OIDC_ISSUER` to turn
kroger-mcp into an OAuth 2.0 **protected resource** (per the MCP authorization
spec, RFC 9728).

kroger-mcp does **not** run any OAuth flows for MCP clients. It delegates
login, consent and token issuance to an external authorization server (e.g.
Authelia) and only:

1. serves protected-resource metadata at `/.well-known/oauth-protected-resource`,
2. answers unauthenticated requests with `401` + `WWW-Authenticate`, and
3. validates the bearer JWT access token (signature via the issuer's JWKS, plus
   `iss` / `aud` / `exp`) on every request.

```sh
export KROGER_MCP_OIDC_ISSUER=https://auth.example.com          # OIDC issuer (discovery base)
export KROGER_MCP_OIDC_RESOURCE=https://kroger-mcp.example.com  # this server's resource id == expected audience
export KROGER_MCP_OIDC_SCOPES="mcp"                             # optional, space-separated
```

The issuer must be reachable at startup, because kroger-mcp runs OIDC discovery then.

### Authorization server requirements (Authelia)

- **JWT access tokens.** Configure the client with
  `access_token_signed_response_alg: RS256`. kroger-mcp validates JWTs, not
  Authelia's default opaque tokens.
- **Audience.** The access token's `aud` must contain the value of
  `KROGER_MCP_OIDC_RESOURCE` (drive this via RFC 8707 resource indicators or the
  client's audience config).
- **Pre-registered client.** Authelia has no Dynamic Client Registration, so
  register a confidential client (authorization code + PKCE `S256`) for Claude.

### Connecting Claude

Claude discovers the auth server from the metadata document, then runs the OAuth
flow. Because Authelia lacks DCR, enter the pre-registered **client ID / secret**
in the connector's **Advanced settings**.

## Config

| Env var                    | Default                                 | Purpose                                                   |
| -------------------------- | --------------------------------------- | --------------------------------------------------------- |
| `KROGER_CLIENT_ID`         | (required)                              | Kroger developer app client ID.                           |
| `KROGER_CLIENT_SECRET`     | (required)                              | Kroger developer app client secret.                       |
| `KROGER_REDIRECT_URI`      | `http://localhost:8080/kroger/callback` | Must match the Kroger app. Its path is the callback route. |
| `KROGER_LOCATION_ID`       | (unset)                                 | Default store when none is saved or passed.               |
| `KROGER_MODALITY`          | `PICKUP`                                | Default cart modality: `PICKUP` or `DELIVERY`.            |
| `KROGER_MCP_ADDR`          | `:8080`                                 | Listen address.                                           |
| `KROGER_MCP_DATA_DIR`      | `data` (`/data` in the image)           | Where `state.json` is kept.                               |
| `KROGER_MCP_LOG_LEVEL`     | `info`                                  | `trace`/`debug`/`info`/`warn`/`error`. Request logs are at `trace`. |
| `KROGER_MCP_OIDC_ISSUER`   | (unset)                                 | OIDC issuer URL. Set to enable MCP auth.                  |
| `KROGER_MCP_OIDC_RESOURCE` | (unset)                                 | Resource identifier / expected token audience.            |
| `KROGER_MCP_OIDC_SCOPES`   | (unset)                                 | Space-separated scopes required on requests.              |

## Releasing

- **Release:** go to **Actions → Release → Run workflow** on `main`, enter a
  version like `v0.1.0`, and run it. It creates the tag, pushes the image
  (`X.Y.Z`, `X.Y`, `latest`), publishes a GitHub release with binaries and
  git-cliff notes, and commits `CHANGELOG.md` to `main`. Pushing a `vX.Y.Z` tag
  yourself does the same.
- **Every merge to `main`** publishes `ghcr.io/philosowaffle/kroger-mcp:main`
  (plus `sha-<commit>`) for testing unreleased changes. No GitHub release is
  created.

## Development

```sh
task test    # go test ./...
task lint    # golangci-lint
task run     # go run . (loads .env)
```

See [`docs/architecture.md`](docs/architecture.md) for how it fits together and
[`docs/runbook.md`](docs/runbook.md) for operations and troubleshooting.
