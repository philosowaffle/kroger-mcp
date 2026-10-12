# Spec: Handle Kroger rate limits (429) gracefully

**Issue:** #27
**Status:** In Progress

## Problem

Kroger's public API enforces per-app daily quotas. Today a 429 comes back as a raw
`kroger api: status 429: …` error, and Claude may simply retry, burning more quota.

## Acceptance Criteria

- [ ] A GET that gets 429 with `Retry-After` retries once and then succeeds.
- [ ] A cart write (`PUT /cart/add`) that gets 429 is not retried.
- [ ] The tool result on quota exhaustion clearly says the quota is used up and not
      to retry right away.
- [ ] Quotas are documented in the README and `docs/api-research.md`.

## Notes

- `Client.do` handles 429 and 503. Only GETs are retried, once, and only when
  `Retry-After` (seconds or an HTTP date) asks for a wait of at most 5 seconds.
  A longer or missing `Retry-After` on a 429 means the daily quota is gone, so
  waiting inside a tool call won't help.
- Cart adds and token requests are never retried: a retried add could add items twice.
- A 429 returns an `*APIError` that matches `errors.Is(err, ErrRateLimited)`; its
  message is written for Claude.
- Quota hits are logged at warn.
- `krogertest.Server.Throttle` makes the fake return 429 on demand.
