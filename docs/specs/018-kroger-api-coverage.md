# Spec: Full Kroger public API coverage and detail levels

**Issue:** none (requested directly; landed in PR #18)
**Status:** Complete

## Problem

The first version of kroger-mcp covered the core shopping flow but skipped the chains
and departments endpoints, several search filters, and most of each product record.
Claude couldn't find valid chain or department filter values, look stores up by
coordinates, or see images, nutrition or store hours when it needed them.

## Acceptance Criteria

- [x] Every public endpoint has a tool: `get_location` (`/locations/{id}`), `list_chains`
      (`/chains`, `/chains/{name}`) and `list_departments` (`/departments`,
      `/departments/{id}`). Covered by `TestLocationTools`, `TestChains`,
      `TestDepartments` and `TestListTools`.
- [x] `search_products` supports `product_ids` and `fulfillment` (friendly names mapped to
      `ais`/`csp`/`dth`/`sth`), and accepts brand or product ids without a term.
      Covered by `TestSearchProducts` and `TestSearchProductsFilters`.
- [x] `search_locations` supports `latitude`/`longitude`, `location_ids` and
      `department_ids`, with validation and radius capped at 100. Covered by
      `TestSearchLocations` and `TestLocationTools`.
- [x] Product and location tools default to lite output. `detail: "full"` returns Kroger's
      complete record, and any other value is rejected. Covered by
      `TestProductDetailLevels`, `TestLocationTools` and `TestWantFull`.
- [x] Product records are cached in memory for 5 minutes per product and store. Searches
      fill the cache, `get_product` reads it, entries expire, and the cache is bounded.
      Covered by `TestGetProductCache`, `TestProductCacheEviction` and
      `TestProductDetailLevels`.

## Notes

- Full records are passed through as-is rather than modelled, so fields Kroger adds later
  (and nutrition, if the public tier returns it) appear without code changes.
- The cache key includes the store, because price and stock differ per store.
- Location records aren't cached: store lookups are rare and cheap.
