# Kroger API Research

This documents findings from pre-build research into the Kroger Public API.
The API was investigated before any live calls were made; notes marked ⚠️ need
validation on the first real run.

## API overview

Kroger (parent company of King Soopers) provides a public developer API at
https://developer.kroger.com. It is actively maintained as of early 2026.

Base URL: `https://api.kroger.com/v1`

## Authentication

The API uses OAuth 2.0 with two grants:

- **Client credentials** (scope `product.compact`) returns an app token. That's
  enough for the Products and Locations APIs, and kroger-mcp keeps it in memory.
- **Authorization Code + PKCE** (scopes `cart.basic:write profile.compact
  product.compact`) returns a user token for the Cart and Identity APIs. It:
  - requires the shopper to log in with their Kroger shopping account once in a browser
  - returns a short-lived access token and a longer-lived refresh token
  - has refresh tokens that expire every 6 months, after which the shopper must re-authorize

kroger-mcp persists the user token in `DATA_DIR/state.json` and refreshes it on demand.

Token endpoint: `https://api.kroger.com/v1/connect/oauth2/token`
Auth endpoint:  `https://api.kroger.com/v1/connect/oauth2/authorize`

## Endpoints used

### Locations API
`GET /v1/locations`

Backs the `search_locations` tool. A store's `locationId` scopes product searches
to that store's catalog, prices and stock.

Parameters used:
- `filter.zipCode.near` — zip code to search near
- `filter.chain` — ⚠️ set to `"King Soopers"` to filter to that chain; **parameter name
  unconfirmed** — if location lookup returns empty results, try removing this filter
- `filter.limit` — number of results to return

`GET /v1/locations/{locationId}` looks up one store; `set_preferred_location` uses it
to validate the id before saving it.

### Products API
`GET /v1/products`

Backs `search_products` (and `GET /v1/products/{id}` backs `get_product`).

Parameters used:
- `filter.term` — the search text
- `filter.locationId` — the store's location ID (ensures results are store-specific)
- `filter.brand` — optional brand filter
- `filter.limit` — 1–50
- `filter.start` — offset for paging

Notes:
- Results are specific to the given store's catalog when `locationId` is provided
- When scoped to a store, items carry `price.regular` / `price.promo`,
  `fulfillment` flags and ⚠️ `inventory.stockLevel` (HIGH / LOW /
  TEMPORARILY_OUT_OF_STOCK). Whether stock level comes back on the public tier
  still needs confirming on a live call.

### Cart API
`PUT /v1/cart/add`

Adds items to the authenticated user's cart. Items appear immediately on kingsoopers.com
and in the King Soopers mobile app.

Request body:
```json
{
  "items": [
    {
      "upc": "0001111041700",
      "quantity": 1,
      "modality": "PICKUP"
    }
  ]
}
```

⚠️ Expected success response is HTTP 204 No Content (possibly 200 OK — to be confirmed on
first run). The code currently accepts either.

## Known limitations of the public API

**Cannot remove items from cart.** The public Cart API is add-only. Removing items requires
either a Partner API contract with Kroger (not publicly available) or manual action on
kingsoopers.com. The cart can't be read back either, so
the `add_to_cart` tool tells Claude to confirm with the shopper before adding.

**No order history.** Past orders are not accessible via the public API.

**Rate limits.** The public Cart API allows 5,000 calls per day, which is far more than
needed for weekly grocery runs.

## References

- Kroger Developer Portal: https://developer.kroger.com
- Kroger Public APIs Postman workspace: https://www.postman.com/kroger/the-kroger-co-s-public-workspace
- Community Python client (useful for cross-referencing params): https://github.com/CupOfOwls/kroger-api
