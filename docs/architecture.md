# Architecture

## Overview

King Soopers Cart Builder is a Go CLI tool with a single, well-defined job: read a
plain-text grocery list, search the Kroger API for each item, and add matched items
to the user's King Soopers pickup cart.

All application code lives in `cmd/kroger-cart/main.go` — a deliberate single-file
design for a tool of this scope. See `docs/decisions/001-single-file-architecture.md`
for the rationale.

## Components

The file is organized into clearly labeled sections:

**Constants** — API URLs, file names, OAuth redirect URI

**Data types** — `Config`, `tokenResponse`, `productItem`, `productsResponse`,
`cartItem`, `cartRequest`

**`main()`** — top-level orchestration: load config → authenticate → resolve location
(first run only) → read grocery list → process each item → print summary

**Config** (`loadConfig`, `firstTimeSetup`, `saveConfig`)
Credentials and tokens are persisted to `config.json` with `0600` permissions (owner
read/write only).

**Authentication** (`ensureAuthenticated`, `doOAuthFlow`, `exchangeCode`, `refreshToken`, `postToken`)
OAuth 2.0 Authorization Code + PKCE flow. On first run, spins up a temporary HTTP server
on `localhost:8080` to catch the redirect callback from Kroger's auth page. The server
shuts down immediately after receiving the code.

**Location lookup** (`resolveLocationID`)
Called once (first run only) when `config.LocationID` is empty. Prompts for zip, queries
the Locations API, lets the user pick their store, then saves the `locationId` to config.

**Product search** (`searchProduct`)
Takes an item name string, calls the Products API with `filter.term` and `filter.locationId`,
returns the first result. Returns `nil, nil` (not an error) when no results are found,
so the caller can distinguish "not found" from "API error".

**Cart** (`addToCart`)
Sends a single `PUT /v1/cart/add` call per item. Items are added with `modality: "PICKUP"`.

**Grocery list** (`readGroceryList`, `createSampleGroceryList`)
Reads `grocery_list.txt` line by line. Lines starting with `#` or empty lines are ignored.
If the file doesn't exist on first run, a sample file is created and the program exits with
a message asking the user to edit it.

**Product cache** (`loadCache`, `saveCache`)
A simple `map[string]string` (lowercased item name → UPC) persisted as JSON.
Cached after every new successful search, so partial runs don't lose progress.
Cache entries for items that fail to add to cart are deleted so they get re-tried next run.

**PKCE helpers** (`generateCodeVerifier`, `generateCodeChallenge`)
Standard PKCE implementation using `crypto/rand` and `crypto/sha256`.

**Utilities** (`openBrowser`, `fatalf`)
`openBrowser` handles Windows (`cmd /c start`), macOS (`open`), and Linux (`xdg-open`).

## Data Flow

```
grocery_list.txt
       │
       ▼
  readGroceryList()
       │
       ▼
  for each item:
       │
       ├─► cache hit? ──yes──► addToCart() ──► ✓ added
       │
       └─► cache miss ──► searchProduct() ──► result?
                               │                  │
                              no                 yes
                               │                  │
                               ▼                  ▼
                         ✗ not found        saveCache()
                                                  │
                                                  ▼
                                            addToCart() ──► ✓ added
                                                        └──► ✗ failed (cache entry removed)
```

## Key Dependencies

**No external Go dependencies** — standard library only. See ADR 001 for the rationale.

Key standard library packages used:

| Package | Purpose |
|---|---|
| `net/http` | HTTP client for all API calls; embedded server for OAuth callback |
| `encoding/json` | JSON marshaling/unmarshaling for API responses and config files |
| `crypto/rand`, `crypto/sha256` | PKCE code verifier and challenge generation |
| `os/exec` | Opening the browser for OAuth authorization |
| `bufio`, `os` | Reading grocery list and config files |
