# kroger-mcp

[![Build](https://github.com/philosowaffle/kroger-mcp/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/philosowaffle/kroger-mcp/actions/workflows/release.yml)
[![Release](https://img.shields.io/github/v/release/philosowaffle/kroger-mcp?sort=semver)](https://github.com/philosowaffle/kroger-mcp/releases)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Image](https://img.shields.io/badge/image-ghcr.io%2Fphilosowaffle%2Fkroger--mcp-2496ED?logo=docker&logoColor=white)](https://github.com/philosowaffle/kroger-mcp/pkgs/container/kroger-mcp)
[![Go version](https://img.shields.io/github/go-mod/go-version/philosowaffle/kroger-mcp)](go.mod)
[![Buy me a coffee](https://img.shields.io/badge/buy%20me%20a%20coffee-FFDD00?logo=buymeacoffee&logoColor=black)](https://buymeacoffee.com/philosowaffle)

**Let Claude build your grocery cart at Kroger-family stores.** A self-hosted
[MCP](https://modelcontextprotocol.io) server for the
[Kroger Public API](https://developer.kroger.com): find stores, search products with
price, promo, stock and aisle, and add items to your cart at King Soopers, Kroger, Ralphs,
Fred Meyer, Fry's, Smith's and other Kroger banners.

> **Unofficial.** kroger-mcp is not affiliated with or endorsed by The Kroger Co.
> See [Disclaimer](#disclaimer).

**Website:** [philosowaffle.github.io/kroger-mcp](https://philosowaffle.github.io/kroger-mcp/),
with a [step-by-step setup guide](https://philosowaffle.github.io/kroger-mcp/setup.html).

## What it looks like

> **You:** Make the King Soopers near zipcode 12345 my store, then add a gallon of 2% milk,
> a dozen large eggs and sourdough bread to my cart for pickup.
>
> **Claude:** *(calls `search_locations`, `set_preferred_location`, then `search_products`
> for each item)* I saved the nearest King Soopers as your store and found Kroger 2% Milk (1 gal,
> $3.49, aisle 12), Simple Truth Large Eggs (12 ct, $4.29, on sale) and Private Selection
> Sourdough ($3.99). Add all three?
>
> **You:** Yes.
>
> **Claude:** *(calls `add_to_cart`)* Added. Open the King Soopers app or website to pick a
> pickup time and check out.

The example is illustrative; prices and products come live from Kroger for your store.

## Quickstart

You need Docker and a Kroger shopping account.

1. **Create a Kroger developer app.** Sign up at <https://developer.kroger.com> (separate
   from your shopping account), then **My Apps → Create App** with the **Product**,
   **Cart**, **Profile** and **Locations** APIs. Add the redirect URI
   `http://localhost:8080/kroger/callback` and copy the **Client ID** and **Client
   Secret**.
2. **Configure.** Download [`docker-compose.yml`](docker-compose.yml) and put a `.env`
   next to it:

   ```sh
   KROGER_CLIENT_ID=your-client-id
   KROGER_CLIENT_SECRET=your-client-secret
   ```

3. **Run it.**

   ```sh
   docker compose up -d     # pulls ghcr.io/philosowaffle/kroger-mcp:latest
   ```

4. **Connect Claude** on the same machine:

   ```sh
   claude mcp add --transport http kroger http://localhost:8080
   ```

5. **Link your account.** Ask Claude to "link my Kroger account", open the URL it gives
   you, sign in with your **shopping** account and approve. Then ask it to find your
   store and make it your default, and you're ready to shop.

The sample compose file only listens on `127.0.0.1`, so the server is reachable from that
machine alone. To use it from claude.ai, the desktop app or your phone, host it somewhere
Claude can reach and turn on [MCP auth](docs/authentication.md#enabling-auth) first.

> [!WARNING]
> **Don't expose the server without MCP auth beyond localhost or a network you trust.**
> With auth off, anyone who can reach the endpoint can add to your cart, or link their own
> Kroger account in place of yours so your later adds land in their cart. The server
> refuses to listen beyond loopback with auth off unless you opt in. See
> [Running without auth](docs/authentication.md#running-without-auth).

## What it can't do

- **Read, change or remove cart items, or check out.** Kroger's public API can only *add*
  to a cart. You finish on the store's website or app.
- **See order history or past purchases.** The API doesn't expose them.
- **Go past Kroger's quotas.** Each developer app gets a daily quota per API: about
  10,000 product calls, 1,600 per location endpoint, and 5,000 each for cart and identity
  (check the [developer portal](https://developer.kroger.com) for current numbers).
- **Run over stdio.** It serves Streamable HTTP only, so it runs as a service that Claude
  connects to.

## Tools

| Tool                     | Needs linked account | What it does                                                        |
| ------------------------ | :------------------: | ------------------------------------------------------------------- |
| `search_locations`       |                      | Find stores by zip code, coordinates or id. Filter by chain and department. |
| `get_location`           |                      | Get one store.                                                      |
| `set_preferred_location` |                      | Save a default store. It persists across restarts.                  |
| `list_chains`            |                      | List chains (valid `chain` filter values), or look one up.          |
| `list_departments`       |                      | List departments (ids for the `department_ids` filter), or look one up. |
| `search_products`        |                      | Search by term, brand or product ids, optionally by fulfillment type. Scoped to a store, it adds price, promo, stock, aisle and fulfillment. |
| `get_product`            |                      | Get one product by product id or UPC. Served from a 5-minute cache when recently seen. |
| `kroger_login`           |                      | Get a single-use URL to link your Kroger shopping account.          |
| `kroger_auth_status`     |                      | Check whether an account is linked and which store is the default.  |
| `add_to_cart`            |          ✓           | Add items to your cart by UPC (PICKUP or DELIVERY).                 |

Product and store tools take `detail`. The default, `lite`, is a compact view for
shopping. `full` returns Kroger's complete record: images, every item and price field,
shelf position, dimensions and nutrition where Kroger provides it for products; hours,
departments and geolocation for stores.

The 5-minute product cache saves quota when Claude asks for details on search results.
When Kroger answers 429, read-only lookups retry once if Kroger asks for a wait of 5
seconds or less. Cart adds are never retried, so items can't be added twice. Otherwise
the tool tells Claude the quota is used up and to try again later.

## Authentication

There are two separate auth layers: optional OAuth on the MCP endpoint (who can use this
server), and Kroger's own OAuth (whose Kroger account it acts on). MCP auth is off by
default. Set `KROGER_MCP_OIDC_ISSUER` to make kroger-mcp an OAuth 2.0 protected resource
that validates JWT access tokens from your own identity provider (e.g. Authelia).

[docs/authentication.md](docs/authentication.md) covers both layers, the risks of running
without auth, identity provider requirements and how account linking works.

## Configuration

| Env var                    | Default                                 | Purpose                                                   |
| -------------------------- | --------------------------------------- | --------------------------------------------------------- |
| `KROGER_CLIENT_ID`         | (required)                              | Kroger developer app client ID.                           |
| `KROGER_CLIENT_SECRET`     | (required)                              | Kroger developer app client secret.                       |
| `KROGER_REDIRECT_URI`      | `http://localhost:8080/kroger/callback` | Must match the Kroger app. Its path is the callback route. |
| `KROGER_LOCATION_ID`       | (unset)                                 | Default store when none is saved or passed.               |
| `KROGER_MODALITY`          | `PICKUP`                                | Default cart modality: `PICKUP` or `DELIVERY`.            |
| `KROGER_MCP_ADDR`          | `127.0.0.1:8080` (`:8080` in the image) | Listen address. Without auth, non-loopback needs the opt-in below. |
| `KROGER_MCP_ALLOW_UNAUTHENTICATED` | `false`                        | Allow listening beyond loopback with auth off. See [Running without auth](docs/authentication.md#running-without-auth). |
| `KROGER_API_BASE_URL`      | `https://api.kroger.com/v1`             | Kroger API root. `https://api-ce.kroger.com/v1` is the certification environment (no shopper accounts, so no cart). |
| `KROGER_MCP_DATA_DIR`      | `data` (`/data` in the image)           | Where `state.json` is kept.                               |
| `KROGER_MCP_LOG_LEVEL`     | `info`                                  | `trace`/`debug`/`info`/`warn`/`error`. Request logs are at `trace`. |
| `KROGER_MCP_OIDC_ISSUER`   | (unset)                                 | OIDC issuer URL. Set to enable MCP auth.                  |
| `KROGER_MCP_OIDC_RESOURCE` | (unset)                                 | Resource identifier / expected token audience.            |
| `KROGER_MCP_OIDC_SCOPES`   | (unset)                                 | Space-separated scopes required on requests. Read from the token's `scope` or `scp` claim. |
| `KROGER_MCP_OIDC_REQUIRE_AT_JWT` | `true`                            | Reject tokens whose `typ` isn't `at+jwt`. Set `false` only if your IdP omits it. |

When you host it somewhere other than your own machine, register that public URL plus
`/kroger/callback` as a redirect URI on the Kroger app and set `KROGER_REDIRECT_URI` to
match it exactly.

Images are published for `linux/amd64` and `linux/arm64`. Releases are tagged `latest`,
`X.Y.Z` and `X.Y`; the tip of `main` is tagged `main`. Release binaries are on the
[releases page](https://github.com/philosowaffle/kroger-mcp/releases). To build from
source, uncomment `build: .` in `docker-compose.yml` and run `docker compose up -d
--build`, or run `go run .`.

## Development

```sh
task test              # go test ./...
task test:integration  # live tests against the Kroger certification API (needs cert credentials)
task lint              # golangci-lint (.golangci.yml)
task ci                # everything PR Checks runs
task run               # go run . (loads .env)
```

See [`docs/architecture.md`](docs/architecture.md) for how it fits together and
[`docs/runbook.md`](docs/runbook.md) for operations, releasing and troubleshooting.
Contributions are welcome: see [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Built with AI assistance

kroger-mcp was built with substantial help from an AI coding assistant (Claude Code). The
maintainer reviews every change before it merges, and the server is covered by end-to-end
tool tests and integration tests against Kroger's certification API.

## Support

If kroger-mcp saves you time, you can
[buy me a coffee](https://buymeacoffee.com/philosowaffle).

## Disclaimer

kroger-mcp is an independent, unofficial project. It is not affiliated with, endorsed by,
or sponsored by The Kroger Co. or any of its banners (King Soopers, Ralphs, Fred Meyer,
Fry's, Smith's, and others). "Kroger" and the banner names are trademarks of their
respective owners and are used here only to describe what the software works with.

kroger-mcp has no Kroger credentials of its own. Each self-hoster registers their own app
on the [Kroger Developer portal](https://developer.kroger.com) and agrees to Kroger's
developer terms, which govern every API call the server makes on their behalf.

## License

[GPL-3.0](LICENSE). You may use, modify and redistribute kroger-mcp under the terms of the
GNU General Public License v3.0.

Report vulnerabilities privately, as described in [SECURITY.md](SECURITY.md).

[Website](https://philosowaffle.github.io/kroger-mcp/) ·
[Setup guide](https://philosowaffle.github.io/kroger-mcp/setup.html) ·
[Security](https://philosowaffle.github.io/kroger-mcp/security.html)
