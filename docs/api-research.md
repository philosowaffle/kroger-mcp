# Kroger API Research

This documents findings from pre-build research into the Kroger Public API.
The API was investigated before any live calls were made; notes marked ⚠️ need
validation on the first real run.

## API overview

Kroger (parent company of King Soopers) provides a public developer API at
https://developer.kroger.com. It is actively maintained as of early 2026.

Base URL: `https://api.kroger.com/v1`

## Authentication

The API uses OAuth 2.0. This tool uses the **Authorization Code + PKCE** flow, which:
- Requires the user to log in with their Kroger shopping account once in a browser
- Returns a short-lived access token and a longer-lived refresh token
- Refresh tokens expire every 6 months; after that the user must re-authorize in the browser

Tokens are cached in `config.json` and refreshed automatically on each run.

Scopes requested: `product.compact cart.basic:write profile.compact`

Token endpoint: `https://api.kroger.com/v1/connect/oauth2/token`
Auth endpoint:  `https://api.kroger.com/v1/connect/oauth2/authorize`

## Endpoints used

### Locations API
`GET /v1/locations`

Used once during first-time setup to find the user's King Soopers store and get its
`locationId`, which is required for scoping product searches.

Parameters used:
- `filter.zipCode.near` — zip code to search near
- `filter.chain` — ⚠️ set to `"King Soopers"` to filter to that chain; **parameter name
  unconfirmed** — if location lookup returns empty results, try removing this filter
- `filter.limit` — number of results to return

The resolved `locationId` is stored in `config.json` and used on every product search.

### Products API
`GET /v1/products`

Used to search for grocery items by name, scoped to the user's store.

Parameters used:
- `filter.term` — the item name from the grocery list
- `filter.locationId` — the store's location ID (ensures results are store-specific)
- `filter.limit` — set to 5; the first result is used and cached

Notes:
- Results are specific to the given store's catalog when `locationId` is provided
- Real-time stock availability is not exposed in the public API tier; the API returns
  product catalog data but not live inventory levels. Out-of-stock items will be flagged
  by King Soopers during the pickup fulfillment process.
- The tool takes the first result and caches it. See the TODO in CLAUDE.md about adding
  an interactive confirmation mode for first-time matches.

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
kingsoopers.com. Practical implication: clear your cart manually before running the tool
if you want a clean slate each week.

**No order history.** Past orders are not accessible via the public API. The product cache
(`product_cache.json`) serves as the tool's substitute — once an item is matched and
confirmed, it's remembered for all future runs.

**Rate limits.** The public Cart API allows 5,000 calls per day, which is far more than
needed for weekly grocery runs.

## References

- Kroger Developer Portal: https://developer.kroger.com
- Kroger Public APIs Postman workspace: https://www.postman.com/kroger/the-kroger-co-s-public-workspace
- Community Python client (useful for cross-referencing params): https://github.com/CupOfOwls/kroger-api
