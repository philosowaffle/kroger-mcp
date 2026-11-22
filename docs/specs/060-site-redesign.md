# Spec: Grocery receipt redesign of the Pages site

**Issue:** #60
**Status:** In Progress

## Problem

The Pages site (#22, #55) works but reads like a default stylesheet: system font, one
blue accent, plain cards and a table. A distinct grocery-receipt look makes the project
easier to remember and share, and turns the example conversation into the centrepiece.

## Acceptance Criteria

- [x] `style.css` defines the tokens for light and dark, and nothing in the HTML hard-codes a color
- [x] Archivo, Inter and JetBrains Mono are self-hosted from `site/fonts/` with their licenses; the site makes no third-party requests
- [x] Home has the receipt hero, aisle-sign cards, grouped tool ledger, stamp notice, CTA band and receipt footer
- [x] Setup has the prerequisites pills, step sidebar, numbered step blocks and labelled code blocks with working Copy buttons (hidden without JS)
- [x] Security uses the same shell
- [x] No emoji in the UI; favicon and `og.png` are updated
- [x] At 375px wide: no horizontal page scroll, hero and sidebar stack, headings don't break mid-word, tap targets are at least 44px
- [x] Text meets WCAG AA in both themes; links and buttons have a visible focus style (2px accent outline, 3px offset)
- [ ] Checked on the live site in light and dark

## Notes

- Built from the issue's spec and its mockups (Home, Setup, Style guide). Still plain
  HTML and CSS in `site/` with no build step, and no change to `pages.yml`.
- Fonts are the Latin subsets of the variable fonts from Fontsource 5.3.0 (Archivo
  with its `wdth` and `wght` axes, Inter and JetBrains Mono with `wght`), about 180 KB
  in all, with each font's OFL license beside it. Characters outside the Latin subset
  (such as →) fall back to the system font.
- Every color is a token on `:root`, redefined under `prefers-color-scheme: dark`.
  The code-block colors (`--block`, `--block-text`, `--block-muted`, `--block-key`,
  `--block-rule`) are tokens too, so the HTML has no inline colors.
- The hero receipt is real text inside a `<figure>` with a visually hidden caption that
  sums it up for screen readers.
- The tool ledger replaces the table. Each row is `name ···· description`; on narrow
  screens the description drops below the name.
- Copy buttons are added by a small inline script on Setup (`navigator.clipboard`), so
  without JavaScript there's no button at all. The `$` prompt isn't copied.
- The Security page's supply-chain line now says CodeQL runs (the repo is public), the
  one copy change.
- `og.png` is drawn with Pillow from the same fonts and tokens.
