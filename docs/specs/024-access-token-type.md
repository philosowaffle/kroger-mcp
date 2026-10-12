# Spec: Reject ID tokens on the MCP endpoint

**Issue:** #24
**Status:** Complete

## Problem

`internal/auth` validated bearer tokens with go-oidc's ID-token verifier, which checks
signature, `iss`, `aud` and `exp` but not the JWT `typ` header. Any RS256 JWT from the
issuer whose `aud` contained the resource was accepted, ID tokens included: a
token-confusion weakness. Scopes were read only from the `scope` claim, so IdPs that
emit `scp` failed the scope check.

## Acceptance Criteria

- [x] A token without `typ: at+jwt` (or `application/at+jwt`) is rejected with 401 by
      default.
- [x] A valid `at+jwt` access token is accepted.
- [x] `KROGER_MCP_OIDC_REQUIRE_AT_JWT=false` turns the `typ` check off for IdPs that
      don't set it.
- [x] Scopes are read from `scope` (space-separated) and `scp` (string or array).
- [x] README documents the setting.

## Notes

- `typ` is read from the JOSE header before go-oidc verifies the token, since
  go-oidc's verifier ignores it. `at+jwt` and `application/at+jwt` are accepted,
  case-insensitively.
- Off-values for `KROGER_MCP_OIDC_REQUIRE_AT_JWT` are `false`, `0`, `no` and `off`;
  anything else, including unset, keeps the check on.
- An unparseable claims payload now rejects the token; it was ignored before.
- Tests: `TestMiddleware_AccessTokenType` (typ on/off, ID token, missing typ),
  `TestMiddleware` (`scp` string and array cases), `TestConfigFromEnv_RequireATJWT`.
