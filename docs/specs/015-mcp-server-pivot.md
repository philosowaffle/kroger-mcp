# Spec: Pivot to a dockerized Kroger MCP server

**Issue:** none (requested directly; landed in PR #15)
**Status:** Complete

## Problem

The CLI matched each grocery-list line to the first search result, which is the part
of shopping that most needs judgment. The goal is to give Claude direct, safe access to
the Kroger API, self-hosted in Docker, with the same MCP auth setup as forge-mcp.

## Acceptance Criteria

- [x] Serves MCP over Streamable HTTP on `KROGER_MCP_ADDR` (default `:8080`). Covered by
      `TestRoutesWithoutAuth`, plus a manual container smoke test.
- [x] Exposes `search_locations`, `set_preferred_location`, `search_products`, `get_product`,
      `kroger_login`, `kroger_auth_status` and `add_to_cart`. Covered by `TestListTools`.
- [x] Product and location tools work with only developer credentials (client-credentials
      token, cached until near expiry). Covered by `TestAppTokenCachedAndRenewed` and
      `TestSearchLocationsAndSetPreferred`.
- [x] Product tools default to the saved store, then `KROGER_LOCATION_ID`. Covered by
      `TestSearchProductsUsesPreferredStore` and `TestResolveLocationID`.
- [x] `kroger_login` returns a single-use, 15-minute, PKCE-backed login URL. The callback
      links the account and persists tokens (0600) across restarts. Covered by
      `TestAuthorizeURL`, `TestCompleteAuthorization` and `TestCallbackHandler`.
- [x] The callback rejects states it didn't issue, expired states and reused states.
      Covered by `TestCompleteAuthorization` and `TestCallbackHandler`.
- [x] User tokens refresh automatically; a rejected refresh unlinks and tells Claude to
      call `kroger_login`. Covered by `TestUserTokenRefresh` and
      `TestExpiredLinkReportedByStatus`.
- [x] `add_to_cart` validates items, applies the default modality and fails clearly when
      not linked. Covered by `TestAddToCart` and `TestAccountAndCartFlow`.
- [x] MCP auth mirrors forge-mcp: it's off unless `KROGER_MCP_OIDC_ISSUER` is set. When on,
      the server serves RFC 9728 metadata, answers 401 + `WWW-Authenticate`, and validates
      the JWT signature/iss/aud/exp and required scopes. Covered by `TestMiddleware`,
      `TestMetadataHandler` and `TestRoutesWithAuth`.
- [x] `/healthz`, `/.well-known/*` and the Kroger callback stay reachable without a bearer
      token when auth is on. Covered by `TestRoutesWithAuth`.
- [x] Distroless nonroot image with a self-probing HEALTHCHECK and a `/data` volume the
      nonroot user can write. Covered by a manual smoke test (container reported
      healthy; volume owned by 65532).

## Notes

- Pending login states live in memory, so a restart invalidates outstanding login URLs.
- Not yet validated against the live API: chain filter values, whether `inventory.stockLevel`
  is returned on the public tier, and whether Kroger honors PKCE (it's harmless if ignored).
- Possible follow-ups: OpenTelemetry (forge-mcp has it), publishing the image to GHCR, and
  a tool to unlink the account.
