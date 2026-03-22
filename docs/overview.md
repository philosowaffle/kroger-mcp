# Overview

## What it does

King Soopers Cart Builder is a Go CLI tool that reads a plain-text grocery list and
automatically adds each item to the user's King Soopers pickup cart via the Kroger Public API.

The user still handles checkout and pickup scheduling manually — this tool only builds the cart.

## How it works at a high level

1. Reads `grocery_list.txt` (one item per line)
2. For each item, checks `product_cache.json` for a previously matched UPC
3. If not cached, searches the Kroger Products API scoped to the user's store
4. Adds the item to the cart via the Kroger Cart API
5. Prints a summary at the end, including anything that failed to be found or added

After the first run, matched products are cached by name so subsequent runs are fast
and don't re-search items that have already been confirmed.

## Files the tool creates at runtime

These are all gitignored and stay local only.

| File | Purpose |
|---|---|
| `config.json` | Kroger API credentials, OAuth tokens, store location ID |
| `product_cache.json` | Maps grocery list item names → UPC codes |
| `grocery_list.txt` | Your grocery list (created as a sample on first run if missing) |

## Setup

### Prerequisites

- Go 1.21 or later — https://go.dev/dl
- A Kroger Developer account and app — https://developer.kroger.com
- Your regular King Soopers account (the one you shop with)

### One-time Kroger developer setup

1. Go to https://developer.kroger.com and create a developer account
   (separate from your shopping account)
2. Once logged in, go to **My Apps → Create App**
3. Give it any name (e.g. `Cart Builder`)
4. Set the Redirect URI to exactly: `http://localhost:8080/callback`
5. Copy your **Client ID** and **Client Secret** — the secret is only shown once

### Building the binary

```bash
cd kroger-cart
go build -o kroger-cart.exe .   # Windows
# or
go build -o kroger-cart .       # Mac/Linux
```

### First run

```bash
.\kroger-cart.exe
```

On first run the tool will:
1. Prompt for your Client ID and Client Secret
2. Open your browser to authorize with Kroger (uses your shopping account login)
3. Prompt for your zip code and show nearby King Soopers stores to pick from
4. Create `config.json` with all of the above saved for future runs

All subsequent runs are silent on auth — tokens are refreshed automatically.

## Running it

1. Edit `grocery_list.txt` with what you need this week
2. Run `.\kroger-cart.exe`
3. Review the summary at the end for anything that needs manual attention
4. Go to kingsoopers.com, verify your cart, then schedule pickup as usual

## Grocery list format

```
# Lines starting with # are comments and are ignored
# One item per line — be as specific as possible for better matching

Bananas
Kroger 2% Reduced Fat Milk gallon
Dave's Killer Bread 21 Whole Grains
Simple Truth Organic Baby Spinach 5oz
```

The more specific the item name, the better the product match. On first run, check
`product_cache.json` to confirm the matched products look right. Edit the UPC values
in that file manually if anything matched wrong, or delete an entry to force a re-search.

## Correcting a bad product match

If an item matched the wrong product, open `product_cache.json` and delete that entry.
The next run will re-search and cache whatever it finds. You can also look up the correct
UPC on kingsoopers.com (it's in the product URL) and paste it directly into the cache file.

## Running with Docker

Docker lets you run the tool without installing Go. Data files are stored in a local
directory and mounted into the container.

### Build the image

```bash
docker build -t kroger-cart .
```

### Run

```bash
mkdir -p data
docker run -it -p 8080:8080 -v "$(pwd)/data:/data" kroger-cart
```

The `data/` directory holds `config.json`, `product_cache.json`, and `grocery_list.txt`.
Put your `grocery_list.txt` there before running.

**OAuth note:** The browser will open on your host machine and redirect to
`localhost:8080`, which is port-mapped to the container. This works as long as
you run the container on the same machine as your browser.

---

## GitHub setup (private repo)

```bash
cd kroger-cart
git init
git add .
git commit -m "Initial commit"
gh repo create kroger-cart --private --source=. --remote=origin --push
```

Or create the repo manually on github.com, then:

```bash
git remote add origin https://github.com/YOUR_USERNAME/kroger-cart.git
git push -u origin main
```
