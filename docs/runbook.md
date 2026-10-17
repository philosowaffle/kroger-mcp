# Runbook

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- Task (`go-task`): https://taskfile.dev/installation/
- A Kroger developer app: https://developer.kroger.com (see README → Setup)

## Local Setup

1. Clone the repo.
2. Create `.env` (gitignored) with at least `KROGER_CLIENT_ID` and
   `KROGER_CLIENT_SECRET`. See the Config table in `README.md`.
3. `task test` to confirm everything passes.
4. `task run` to start on `127.0.0.1:8080`, with state in `./data/`.
5. `claude mcp add --transport http kroger http://localhost:8080`

MCP auth is off unless `KROGER_MCP_OIDC_ISSUER` is set, so the server fails closed:
without auth it only listens on loopback. Setting `KROGER_MCP_ADDR` to a non-loopback
address (e.g. `:8080`) makes it refuse to start unless auth is on or
`KROGER_MCP_ALLOW_UNAUTHENTICATED=true`. See README → Running without auth.

For the Docker path, `task docker:up` pulls and starts the published image
(compose reads `.env`). State is kept in the `kroger-data` volume. The sample
compose file publishes the port on `127.0.0.1` only and sets
`KROGER_MCP_ALLOW_UNAUTHENTICATED=true`, because inside the container the server
has to listen on `:8080`. Remove that variable once MCP auth is configured. To run
local source in Docker, uncomment `build: .` in `docker-compose.yml` and run
`docker compose up -d --build`.

## Common Tasks

See `Taskfile.yml`. Key ones:

```bash
task build         # compile to bin/kroger-mcp
task run           # go run . with .env loaded
task test          # unit + in-memory end-to-end tests
task test:integration  # live tests against the Kroger certification API
task lint          # golangci-lint (config in .golangci.yml)
task ci            # what PR Checks runs: fmt check, vet, lint, build, test
task docker:build  # build image
task docker:up     # docker compose up -d (published image)
task docker:down   # docker compose down
```

## Integration Tests

`integration/` holds tests that call a live Kroger API, gated behind the
`integration` build tag so `go test ./...` never runs them. By default they hit
the certification environment (`https://api-ce.kroger.com/v1`). They cover
locations, chains, departments and products through the client, and every
read-only tool end to end through a real MCP session.

The cart and identity APIs aren't covered: certification has no shopper
accounts, and production would mean writing to a real cart.

Run them locally with certification credentials in `.env`:

```bash
KROGER_CLIENT_ID=...       # a certification app, not your production one
KROGER_CLIENT_SECRET=...
# optional: KROGER_TEST_ZIP=45202  (zip to search near)
task test:integration
```

Without credentials the tests skip.

In CI, the `integration` job in `pr-checks.yml` runs them on every PR, once
the `checks` job (format, vet, lint, build, unit tests, Docker build) has passed. It
needs two repository secrets: `KROGER_CERT_CLIENT_ID` and
`KROGER_CERT_CLIENT_SECRET`. A repository variable `KROGER_TEST_ZIP` overrides
the default zip. If the secrets are missing (as on PRs from forks), the job
passes with a warning instead of failing. The `checks` job vets the
integration code either way, so it can't silently stop compiling.

## Releasing

1. Merge to `main` with Conventional Commit titles, since git-cliff builds the
   notes from them. Each merge also publishes a `main` image
   (`ghcr.io/philosowaffle/kroger-mcp:main`) you can test before releasing.
2. Go to **Actions → Release → Run workflow** with branch `main`, enter the
   version (`vX.Y.Z`), and run it. Pushing a `vX.Y.Z` tag from a terminal also
   works.
3. The workflow creates the tag (manual runs only), pushes the image
   (`X.Y.Z`, `X.Y`, `latest`) for amd64 and arm64, creates the GitHub release
   with binaries and a `SHA256SUMS` file, and then commits `CHANGELOG.md` to
   `main` in a separate job.

Binaries are static (`CGO_ENABLED=0`) and built with `-trimpath`. The image ships an
SBOM and max-mode build provenance. Once the repo is public, the workflow also adds
GitHub build-provenance attestations for every binary and the image. They're skipped
while it's private, because attestations need a public repo or GitHub Enterprise Cloud.
To verify a download:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify kroger-mcp-linux-amd64 --repo philosowaffle/kroger-cart
gh attestation verify oci://ghcr.io/philosowaffle/kroger-mcp:vX.Y.Z --repo philosowaffle/kroger-cart
```

Release builds stamp the tag and commit into the binary (`-ldflags -X`), so
`kroger-mcp -version` and the MCP `serverInfo.version` report e.g. `v0.1.0`;
`main` images report `main`. `task build` uses `git describe`, and a plain
`go build` reports `dev`.

A manual run fails early, before anything is published, if the version isn't
`vX.Y.Z`, the tag already exists, or it wasn't started from `main`.

The first push creates the GHCR package as **private**. Either make it public
(the package page → Package settings → Change visibility) or
`docker login ghcr.io` on the host with a PAT that has `read:packages`.

If `main` is protected, the changelog job's push fails. The release and image
are unaffected. Allow `github-actions[bot]` to bypass the protection, or drop
that job.

## CI and dependency updates

- Every action in `.github/workflows/` is pinned to a commit SHA with a
  `# vX.Y.Z` comment. Dependabot (`.github/dependabot.yml`) opens weekly grouped
  PRs for actions, Go modules and Docker base images, and updates the SHA and
  the comment together. Don't hand-edit a pin back to a tag.
- Workflows install the newest Go 1.26 patch (`go-version: "1.26.x"`,
  `check-latest: true`) rather than the `go` line in `go.mod`, which is only the
  minimum. That way CI, govulncheck and release binaries pick up stdlib security fixes.
- golangci-lint is pinned in `pr-checks.yml`. To bump it, change the `version:`
  there and the version you run locally in the same PR.
- The **Security** workflow runs `govulncheck` on PRs, on `main` and weekly, and
  CodeQL on PRs and `main`. CodeQL is skipped while the repo is private, because
  uploading code-scanning results there needs GitHub Advanced Security.

## Runtime State

`$KROGER_MCP_DATA_DIR/state.json` (`/data/state.json` in the container) holds the
linked shopper's access and refresh tokens and the preferred store. It is
written with mode 0600. Back it up if you don't want to relink; delete it to
unlink.

## Troubleshooting

**Server exits with `refusing to listen on ":8080" without auth`**
`KROGER_MCP_ADDR` isn't a loopback address and MCP auth is off. Set
`KROGER_MCP_OIDC_ISSUER` (and `KROGER_MCP_OIDC_RESOURCE`) to require OAuth, listen
on `127.0.0.1:8080`, or set `KROGER_MCP_ALLOW_UNAUTHENTICATED=true` if the network
is trusted.

**Tools say the Kroger API quota is used up (429)**
Kroger enforces per-app daily quotas. Reads retry once when Kroger asks for a short
wait; cart adds never retry. Otherwise wait for the quota to reset.

**`add_to_cart` says the account isn't linked**
Ask Claude to run `kroger_login` and open the URL. Kroger refresh tokens last
about six months. When a refresh is rejected, the server clears the link and
asks you to link again.

**Callback page says "Link expired"**
Login URLs are single-use and expire after 15 minutes. They also don't survive
a server restart, because pending states live in memory. Ask for a new one.

**Kroger shows a redirect_uri error on the login page**
`KROGER_REDIRECT_URI` must match a redirect URI on the Kroger developer app
character for character, including scheme, host, port and path.

**Callback page says something went wrong exchanging the code**
Check the logs (`docker logs kroger-mcp`) for the Kroger error. The usual cause
is a wrong client secret.

**`permission denied` writing state in Docker**
The container runs as uid 65532. Named volumes inherit the right ownership. For
a bind mount, `chown 65532:65532` the host directory.

**Product results have no price or stock**
No store was in scope. Set one with `set_preferred_location` or
`KROGER_LOCATION_ID`, or pass `location_id`.

**Claude gets 401 from the MCP endpoint (auth on)**
The server logs the exact rejection at WARN. Common causes: the token `aud`
doesn't contain `KROGER_MCP_OIDC_RESOURCE`, the provider issued an opaque token
instead of a JWT, or the issuer doesn't match. Set `KROGER_MCP_LOG_LEVEL=trace`
for per-request logs.

**Location search returns nothing with a chain filter**
`chain` has to match Kroger's chain name exactly. Ask Claude to run
`list_chains` and use a name from there.
