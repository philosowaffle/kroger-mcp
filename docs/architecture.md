# Architecture

## Language and dependencies

Written in Go 1.21. No external dependencies — standard library only.
This is intentional; keeping it dependency-free means no `go mod tidy`, no supply chain
concerns, and easy builds on any machine with Go installed.

## Project structure

```
kroger-cart/
├── main.go               # All application code (single-file by design)
├── go.mod                # Module definition, no external deps
├── CLAUDE.md             # AI assistant orientation and TODO list
├── .gitignore            # Excludes credentials and runtime files from git
├── docs/
│   ├── overview.md       # User-facing setup and usage guide
│   ├── api-research.md   # Kroger API findings and limitations
│   └── architecture.md   # This file
│
# Runtime files (gitignored, created on first run):
├── config.json           # Credentials, OAuth tokens, location ID
├── product_cache.json    # Item name → UPC mapping
└── grocery_list.txt      # The user's grocery list
```

All runtime files are expected to live in the same directory as the binary.

## Code structure (main.go)

The file is organized into clearly labeled sections:

**Constants** — API URLs, file names, OAuth redirect URI

**Data types** — `Config`, `tokenResponse`, `productItem`, `productsResponse`,
`cartItem`, `cartRequest`

**`main()`** — top-level orchestration: load config → authenticate → resolve location
(first run only) → read grocery list → process each item → print summary

**Config** — `loadConfig`, `firstTimeSetup`, `saveConfig`
Credentials and tokens are persisted to `config.json` with `0600` permissions (owner read/write only).

**Authentication** — `ensureAuthenticated`, `doOAuthFlow`, `exchangeCode`, `refreshToken`, `postToken`
OAuth 2.0 Authorization Code + PKCE flow. On first run, spins up a temporary HTTP server
on `localhost:8080` to catch the redirect callback from Kroger's auth page. The server
shuts down immediately after receiving the code.

**Location lookup** — `resolveLocationID`
Called once (first run only) when `config.LocationID` is empty. Prompts for zip, queries
the Locations API, lets the user pick their store, then saves the `locationId` to config.

**Product search** — `searchProduct`
Takes an item name string, calls the Products API with `filter.term` and `filter.locationId`,
returns the first result. Returns `nil, nil` (not an error) when no results are found,
so the caller can distinguish "not found" from "API error".

**Cart** — `addToCart`
Sends a single `PUT /v1/cart/add` call per item. Items are added with `modality: "PICKUP"`.

**Grocery list** — `readGroceryList`, `createSampleGroceryList`
Reads `grocery_list.txt` line by line. Lines starting with `#` or empty lines are ignored.
If the file doesn't exist on first run, a sample file is created and the program exits with
a message asking the user to edit it.

**Product cache** — `loadCache`, `saveCache`
A simple `map[string]string` (lowercased item name → UPC) persisted as JSON.
Cached after every new successful search, so partial runs don't lose progress.
Cache entries for items that fail to add to cart are deleted so they get re-tried next run.

**PKCE helpers** — `generateCodeVerifier`, `generateCodeChallenge`
Standard PKCE implementation using `crypto/rand` and `crypto/sha256`.

**Utilities** — `openBrowser`, `fatalf`
`openBrowser` handles Windows (`cmd /c start`), macOS (`open`), and Linux (`xdg-open`).

## Data flow

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

## Design decisions and trade-offs

**Single file.** All code lives in `main.go`. For a personal tool of this scope, splitting
into packages adds navigation overhead with no real benefit. If the tool grows significantly
(multiple subcommands, tests, etc.) it would make sense to split then.

**First-result caching.** Product search takes the first API result and caches it without
asking the user. This is fast and works well for specific item names, but can produce wrong
matches for ambiguous terms. The planned `--confirm` flag (see CLAUDE.md TODO) would add
an interactive confirmation step for first-time matches.

**Add-only cart.** The Kroger public API cannot remove items from a cart. The tool is
designed around this — it's an additive operation. The user is expected to clear their
cart manually before each run if they want a clean slate.

**Credentials in config.json.** The `client_secret` and OAuth tokens are stored in plain
text in `config.json` with `0600` permissions. For a personal CLI tool this is the standard
approach (same pattern as `~/.npmrc`, `~/.aws/credentials`, etc.). The file is gitignored.
