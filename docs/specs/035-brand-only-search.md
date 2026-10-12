# Spec: Brand-only product search

**Issue:** #35
**Status:** Complete

## Problem

`search_products` accepts `brand` on its own, but Kroger rejects a product search
that has neither `filter.term` nor `filter.productId` with `PRODUCT-2016: Field
'term' or 'productId' must be used`. `filter.brand` is only a filter on top of one
of those, so a brand-only call fails with a raw API error.

## Acceptance Criteria

- [x] A brand-only search sends the brand as `filter.term` (and keeps `filter.brand`),
      so Kroger accepts it and returns that brand's products.
- [x] A search with a term or product ids is unchanged: the brand stays a filter only.
- [x] A search with none of term, product ids or brand is still rejected before
      calling Kroger.
- [x] The fake Kroger API rejects a product search without a term or product id,
      as the real API does, so the tests catch a regression.

## Notes

- The issue offered two fixes: require a term or ids, or use the brand as the term.
  Using the brand as the term keeps "show me Simple Truth products" working in one
  call, which is what a shopper means.
- The tool schema text says brand alone works as a search.
- Tests: `TestSearchProducts` ("brand alone searches by brand", "brand with product
  ids stays a filter", "nothing to search for") and `TestSearchProducts_BrandAlone`
  over MCP. The fake's `/products` now returns PRODUCT-2016 for a search with no
  term or product id.
