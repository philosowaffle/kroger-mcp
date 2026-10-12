# Spec: Reject ID tokens on the MCP endpoint

**Issue:** #24
**Status:** In Progress

## Problem

`internal/auth` validated bearer tokens with go-oidc's ID-token verifier, which checks
signature, `iss`, `aud` and `exp` but not the JWT `typ` header. Any RS256 JWT from the
issuer whose `aud` contained the resource was accepted, ID tokens included: a
token-confusion weakness. Scopes were read only from the `scope` claim, so IdPs that
emit `scp` failed the scope check.

## Acceptance Criteria

- [ ] A token without `typ: at+jwt` (or `application/at+jwt`) is rejected with 401 by
      default.
- [ ] A valid `at+jwt` access token is accepted.
- [ ] `KROGER_MCP_OIDC_REQUIRE_AT_JWT=false` turns the `typ` check off for IdPs that
      don't set it.
- [ ] Scopes are read from `scope` (space-separated) and `scp` (string or array).
- [ ] README documents the setting.

## Notes
