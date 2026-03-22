# CLAUDE.md — King Soopers Cart Builder

This file is the entry point for any AI assistant picking up this project.
Read this first, then inspect the `docs/` folder for full context and specs before touching any code.

## What this project is

A Go CLI tool that automates building a King Soopers grocery cart via the Kroger Public API.
The user provides a plain-text grocery list; the tool searches for each item, adds it to the
live King Soopers cart (visible immediately on kingsoopers.com), and reports anything it
couldn't find. Checkout and pickup scheduling remain manual.

## Docs to read before working on this

| File | What's in it |
|---|---|
| `docs/overview.md` | User-facing overview, setup steps, how to run |
| `docs/api-research.md` | Kroger API findings, endpoints used, known limitations |
| `docs/architecture.md` | Code structure, design decisions, data flow |

## Current state

The initial implementation is complete and ready for a first live test run.
No real API calls have been made yet — all design is based on API documentation research.
The first run will validate assumptions and likely surface minor adjustments needed.

## Owner

Bailey — bailey@... — personal project, private GitHub repo.

---

## TODO

Things that have been planned or discussed but are not yet implemented.
Work through these roughly in order; items marked with ⚠️ should be validated first.

### Must do before relying on this tool

- [ ] ⚠️ **First live test run** — run the tool end-to-end with real credentials and a short
  grocery list. Validate that OAuth, product search, and cart add all work as expected.

- [ ] ⚠️ **Verify `filter.chain` parameter** — the Locations API call uses
  `filter.chain=King Soopers` to scope results. This parameter name is inferred from docs;
  if the location lookup returns empty results or errors, try removing that filter and
  picking your store from whatever comes back. Update the code once confirmed.

- [ ] ⚠️ **Verify cart API response code** — `addToCart` accepts HTTP 200 or 204 as success.
  Confirm which one the Kroger API actually returns and tighten the check if needed.

### Quality of life improvements

- [ ] **Interactive match review on first run** — right now the first product search result
  is cached and used automatically. Consider an interactive mode (`--confirm`) that prints
  the matched product and asks "Is this right? [y/n]" before caching, so bad matches get
  caught before they're locked in.

- [ ] **Quantity support in grocery list** — currently all items are added with quantity 1.
  Support a syntax like `2x Bananas` or `Bananas x2` in `grocery_list.txt` so quantities
  can be specified per item.

- [ ] **`--dry-run` flag** — search for all items and print what would be added, without
  actually calling the cart API. Useful for previewing before committing.

- [ ] **`--recache` flag** — re-run product search for all items (ignoring the cache) so
  the user can refresh matches without manually editing `product_cache.json`.

- [ ] **`--item` flag** — add a single ad-hoc item without editing `grocery_list.txt`.
  Example: `kroger-cart --item "Tillamook Cheddar 2lb"`.

### Infrastructure

- [ ] **Push to GitHub** — create private repo, push initial commit. See setup instructions
  in `docs/overview.md`.
