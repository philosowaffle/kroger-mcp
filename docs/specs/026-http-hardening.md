# Spec: Harden the HTTP surface

**Issue:** #26
**Status:** In Progress

## Problem

Defense-in-depth gaps in the HTTP server and the Kroger OAuth callback: no
cross-origin protection on the MCP endpoint off loopback, MCP sessions that never
expire, no idle timeout or header size limit, an unbounded map of pending logins,
no security headers on the callback page, and an unbounded query value logged and
echoed from the callback.

## Acceptance Criteria

- [ ] A cross-origin browser POST to the MCP endpoint is rejected with 403; non-browser
      clients (no `Origin`/`Sec-Fetch-Site`) are unaffected.
- [ ] MCP sessions expire after an idle timeout.
- [ ] `http.Server` sets `IdleTimeout` and `MaxHeaderBytes`.
- [ ] The pending-login map is capped; the oldest entry is evicted when full.
- [ ] Every callback response carries `Content-Security-Policy`, `Referrer-Policy:
      no-referrer`, `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`.
- [ ] The callback's `error` and `error_description` values are truncated before being
      logged or rendered.

## Notes
