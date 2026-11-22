# Spec: GitHub Pages landing site

**Issue:** #22
**Status:** In Progress

## Problem

A small static site gives the project a shareable home that's friendlier than a README,
and a natural place for a setup guide.

## Acceptance Criteria

- [ ] The site deploys from `main` via Actions and is reachable at
      `https://philosowaffle.github.io/kroger-mcp/` (after merge, once Pages is on)
- [ ] It works at phone width and in both light and dark themes (built for it; check on
      the live site)
- [x] The footer includes BuyMeACoffee, the AI-assistance note, the disclaimer and the license
- [x] The workflow passes the CI hardening rules (pinned SHAs, minimal permissions)

## Notes

- Plain HTML and CSS in `site/` with no build step: `index.html` (pitch, example
  conversation, tools, "can't check out" callout), `setup.html` (five-step guide plus
  hosting for other devices), `security.html` (auth layers, running without auth,
  linking, stored tokens, reporting), a shared `style.css`, `favicon.svg` and a
  1200×630 `og.png` for Open Graph and Twitter cards.
- Light and dark themes follow `prefers-color-scheme` through CSS variables. The layout
  is a single 860px column with a 16px gutter; tables and code blocks scroll sideways
  instead of widening the page.
- `.github/workflows/pages.yml` runs on pushes to `main` that touch `site/**` or the
  workflow, and on manual dispatch. `permissions: contents: read` at the top; only the
  deploy job gets `pages: write` and `id-token: write`. Actions are pinned to SHAs:
  configure-pages v6.0.0, upload-pages-artifact v5.0.0, deploy-pages v5.0.1.
- Maintainer steps after merge: Settings → Pages → Source "GitHub Actions", then run the
  workflow (or push a `site/` change). GitHub Pages on a private repo needs a paid plan,
  so on a free plan the site goes live when the repo is made public (#33).
- Still to do after it's live: set the repo homepage to the site (#33) and link the site
  from the README footer.
- No screenshots of the Kroger developer portal yet; the setup guide describes the
  steps in text.
