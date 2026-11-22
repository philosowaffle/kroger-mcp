# Spec: Harden the HTTP surface

**Issue:** #26
**Status:** Complete

## Problem

Defense-in-depth gaps in the HTTP server and the Kroger OAuth callback: no
cross-origin protection on the MCP endpoint off loopback, MCP sessions that never
expire, no idle timeout or header size limit, an unbounded map of pending logins,
no security headers on the callback page, and an unbounded query value logged and
echoed from the callback.

## Acceptance Criteria

- [x] A cross-origin browser POST to the MCP endpoint is rejected with 403; non-browser
      clients (no `Origin`/`Sec-Fetch-Site`) are unaffected.
- [x] MCP sessions expire after an idle timeout.
- [x] `http.Server` sets `IdleTimeout` and `MaxHeaderBytes`.
- [x] The pending-login map is capped; the oldest entry is evicted when full.
- [x] Every callback response carries `Content-Security-Policy`, `Referrer-Policy:
      no-referrer`, `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`.
- [x] The callback's `error` and `error_description` values are truncated before being
      logged or rendered.

## Notes

- Cross-origin protection is Go's `http.CrossOriginProtection`, wrapped outside the
  auth middleware so a cross-site request gets 403 before any token handling. It
  only acts on browser signals (`Sec-Fetch-Site`, `Origin`), so Claude Code and
  claude.ai's connector backend are unaffected. Test: `TestCrossOriginRejected`.
- `SessionTimeout` was one hour. Superseded by #48: the handler is now stateless, so
  there are no MCP sessions to expire (go-sdk rejected calls carrying the
  protocolVersion `_meta` key on stateful servers).
- `ReadTimeout` and `WriteTimeout` stay unset on purpose: both cut off long-lived SSE
  streams. The SDK already caps request bodies at 4 MiB.
- At most 32 pending logins; when full, the one closest to expiry is evicted. Test:
  `TestAuthorizeURL_CapsPendingLogins`.
- Callback headers and truncation (200 runes) are covered in `TestCallbackHandler`
  and `TestTruncate`.
- Smoke-tested against the built binary: a plain MCP `initialize` returns 200, a
  `Sec-Fetch-Site: cross-site` one returns 403.
