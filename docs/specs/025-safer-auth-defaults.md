# Spec: Safer defaults when MCP auth is off

**Issue:** #25
**Status:** In Progress

## Problem

MCP auth is off by default, and the sample `docker-compose.yml` publishes `"8080:8080"`,
which binds every host interface. Anyone who can reach the port can call `add_to_cart`
on the shopper's account, or call `kroger_login`, sign in with *their own* Kroger
account and silently replace the linked one (account substitution). The single-use
state stops a drive-by callback, but not someone who can call the MCP tool.

## Acceptance Criteria

- [ ] Default compose binds only to loopback (`127.0.0.1:8080:8080`).
- [ ] With auth off, the server refuses to start on a non-loopback listen address unless
      `KROGER_MCP_ALLOW_UNAUTHENTICATED=true`; a test covers start-up on loopback,
      non-loopback, and non-loopback with the opt-in.
- [ ] `kroger_login` output warns when completing it would replace an existing link.
- [ ] A link that replaces a different profile is logged at info with both profile IDs.
- [ ] README and SECURITY notes explain the risk and the opt-in.

## Notes

- **Fail closed, not warn.** This is pre-v0.1.0, so the behaviour change is cheap now and
  costly later. Release notes must call it out for existing LAN setups.
- Loopback means the host part of `KROGER_MCP_ADDR` is `localhost` or a loopback IP. An
  empty host (`:8080`), `0.0.0.0`, `::` and any other hostname count as exposed.
- Inside a container the server must listen on `:8080` to be reachable through the port
  mapping, so the compose file sets `KROGER_MCP_ALLOW_UNAUTHENTICATED=true` *together with*
  the loopback-only port publish, with a comment. A bare `docker run` with neither auth
  nor the opt-in refuses to start, which is the point.
- The profile ID is fetched once after each successful link (best effort; a failure
  doesn't fail the link) and stored in `state.json`, so a later link can tell whether it
  replaced a different account.
