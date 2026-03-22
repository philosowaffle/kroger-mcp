# Runbook

## Prerequisites

- Go 1.21+
- Docker + Docker Compose
- Task (`go-task`) — install: https://taskfile.dev/installation/
- A Kroger Developer account and app — https://developer.kroger.com

## Local Setup

1. Clone the repo
2. Run `task build` to compile the binary to `bin/kroger-cart`
3. On first run the tool will prompt for your Kroger API credentials,
   open your browser for OAuth authorization, and ask for your zip code
   to find your nearest King Soopers store
4. Edit `grocery_list.txt` with this week's items
5. Run `bin/kroger-cart` (or `.\bin\kroger-cart.exe` on Windows)

## First-Run OAuth Flow

The tool uses OAuth 2.0 + PKCE. On first run:

1. Your browser opens to the Kroger authorization page — log in with your **shopping** account
2. After authorizing, Kroger redirects to `localhost:8080/callback`
3. The tool catches the callback, exchanges the code for tokens, and saves them to `config.json`

If your browser doesn't open automatically, copy the URL printed to the terminal.

## Runtime Files

These are gitignored and stay local only.

| File | Purpose |
|---|---|
| `config.json` | Kroger API credentials, OAuth tokens, store location ID |
| `product_cache.json` | Maps grocery list item names → UPC codes |
| `grocery_list.txt` | Your grocery list (created as a sample on first run if missing) |

All runtime files are expected to live in the working directory where the binary is run.

## Running with Docker

```bash
task docker:build

mkdir -p data
# place grocery_list.txt in data/ before running
docker-compose up
```

The `data/` directory is mounted into the container at `/data`. Port 8080 is mapped for OAuth.

## Common Tasks

See `Taskfile.yml` for all available tasks. Key ones:

```bash
task build         # compile binary to bin/kroger-cart
task fmt           # format all Go files
task vet           # run go vet
task lint          # run golangci-lint
task test          # run unit tests
task docker:build  # build Docker image
task docker:up     # start via docker-compose
task docker:down   # stop docker-compose
```

## Correcting a Bad Product Match

Open `product_cache.json` and delete the entry for the mismatched item.
The next run will re-search and cache whatever it finds.
You can also look up the correct UPC on kingsoopers.com (it's in the product URL)
and paste it directly into the cache file.

## Troubleshooting

**Location lookup returns no results**
The `filter.chain=King Soopers` parameter may not be supported by the live API.
Try removing it from `resolveLocationID()` in `cmd/kroger-cart/main.go` and selecting
your store from the full results list.

**OAuth callback never received**
Ensure `http://localhost:8080/callback` is set as the redirect URI in your Kroger developer app.
Port 8080 must be free when running the tool.

**Token expired**
Delete `config.json` and re-run to go through the full auth flow again.
