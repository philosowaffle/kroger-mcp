# Spec: Basic integration tests against the Kroger certification API

**Issue:** none (requested directly)
**Status:** Complete

## Problem

Unit tests run against a fake Kroger API, so nothing catches a mismatch with the real API's
response shapes, auth or filters until someone uses the server. The goal is a basic live
check on every PR that can be expanded later.

## Acceptance Criteria

- [x] Integration tests live in `integration/` behind the `integration` build tag, so
      `go test ./...` never runs them.
- [x] They target the certification environment by default (`KROGER_API_BASE_URL`
      overrides it) and skip when credentials aren't set.
- [x] The client is covered against the live API: location search and lookup, chains,
      departments, product search (term, product ids, fulfillment filter) and product
      lookup (`TestLocations`, `TestChainsAndDepartments`, `TestProducts`).
- [x] Every read-only MCP tool runs end to end through a real MCP session, including
      `detail: "full"`, so live responses are decoded and schema-validated (`TestMCPTools`).
- [x] A PR check (`integration` job) runs them with the `KROGER_CERT_CLIENT_ID` /
      `KROGER_CERT_CLIENT_SECRET` secrets. It warns and passes when the secrets are absent.
- [x] The `checks` job vets the integration code so it keeps compiling without secrets.
- [x] `KROGER_API_BASE_URL` lets the server itself run against certification.

## Notes

- Cart and identity are out of scope: certification has no shopper accounts, and
  production would write to a real cart.
- The default zip is 45202. If certification has no stores there, set the `KROGER_TEST_ZIP`
  repository variable.
- The tests couldn't be run live from the development sandbox (its network policy blocks
  `api-ce.kroger.com`). Their logic was checked against the in-repo fake, and the first real
  run is in CI.
