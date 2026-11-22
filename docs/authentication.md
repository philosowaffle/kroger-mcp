# Authentication

kroger-mcp has two separate auth layers, and it helps to keep them apart:

1. **Who can use this MCP server** (Claude → kroger-mcp). This is optional OAuth
   protection of the MCP endpoint, delegated to your own identity provider. It's
   described below.
2. **Whose Kroger account it acts on** (kroger-mcp → Kroger). This uses Kroger's own
   OAuth. Product and store lookups use an app token from your Kroger developer
   credentials. The cart needs your shopping account, which you link once in a browser
   (see [Linking your Kroger account](#linking-your-kroger-account)).

## Running without auth

MCP auth is off by default. Without it, **anyone who can reach the MCP endpoint can use
your linked Kroger account**: add items to your cart, or call `kroger_login`, sign in
with their own Kroger account and replace yours, so your later cart adds land in their
cart. The single-use login state stops a drive-by callback, but not someone who can call
the tool.

So with auth off the server only listens on loopback by default:

- If `KROGER_MCP_ADDR` isn't a loopback address (`127.0.0.1:8080`, `localhost:8080`,
  `[::1]:8080`), it **refuses to start** unless `KROGER_MCP_ALLOW_UNAUTHENTICATED=true`
  is set. Set that only on a network you trust (a home LAN, a VPN, or behind a reverse
  proxy that does its own auth).
- The sample `docker-compose.yml` publishes the port on `127.0.0.1` only. It sets
  `KROGER_MCP_ALLOW_UNAUTHENTICATED=true` because the server must listen on all
  interfaces *inside* the container; the loopback-only publish is what keeps it private.
- `kroger_login` warns when an account is already linked, and the server logs
  `kroger: linked account replaced` at info with both profile IDs when a link swaps
  accounts.

## Enabling auth

Set `KROGER_MCP_OIDC_ISSUER` to turn kroger-mcp into an OAuth 2.0 **protected resource**
(per the MCP authorization spec, RFC 9728).

```sh
export KROGER_MCP_OIDC_ISSUER=https://auth.example.com          # OIDC issuer (discovery base)
export KROGER_MCP_OIDC_RESOURCE=https://kroger-mcp.example.com  # this server's resource id == expected audience
export KROGER_MCP_OIDC_SCOPES="mcp"                             # optional, space-separated
```

The issuer must be reachable at startup, because kroger-mcp runs OIDC discovery then.

kroger-mcp does **not** run any OAuth flows for MCP clients. It delegates login, consent
and token issuance to an external authorization server (e.g. Authelia) and only:

1. serves protected-resource metadata at `/.well-known/oauth-protected-resource`,
2. answers unauthenticated requests with `401` + `WWW-Authenticate`, and
3. validates the bearer JWT access token (signature via the issuer's JWKS, plus
   `iss` / `aud` / `exp`, and a `typ` header of `at+jwt` per RFC 9068) on every
   request. The `typ` check stops an ID token for the same audience from being used as
   an access token.

### Authorization server requirements (Authelia)

- **JWT access tokens.** Configure the client with
  `access_token_signed_response_alg: RS256`. kroger-mcp validates JWTs, not Authelia's
  default opaque tokens.
- **Token type.** Access tokens must carry the JOSE header `typ: at+jwt`. If your
  authorization server omits it, set `KROGER_MCP_OIDC_REQUIRE_AT_JWT=false` (the server
  logs `jwt typ "..." is not at+jwt` when it rejects one).
- **Audience.** The access token's `aud` must contain the value of
  `KROGER_MCP_OIDC_RESOURCE` (drive this via RFC 8707 resource indicators or the
  client's audience config).
- **Pre-registered client.** Authelia has no Dynamic Client Registration, so register a
  confidential client (authorization code + PKCE `S256`) for Claude.

### Connecting Claude

Claude discovers the auth server from the metadata document, then runs the OAuth flow.
Because Authelia lacks DCR, enter the pre-registered **client ID / secret** in the
connector's **Advanced settings**.

## Linking your Kroger account

Ask Claude something like "link my Kroger account". It calls `kroger_login` and gives you
a URL. Open it, sign in with your **shopping** account and approve. Kroger redirects to
`/kroger/callback`, and the server stores the tokens in `state.json` (in
`KROGER_MCP_DATA_DIR`, `/data` in the image) and refreshes them automatically. Links last
about six months; after that, `add_to_cart` asks you to link again.

The callback is reachable without MCP auth, because a browser redirect can't carry a
bearer token. It only accepts the single-use `state` from a `kroger_login` URL issued in
the last 15 minutes, so nobody can use it to link an account without first getting a
login URL through an authenticated MCP call.
