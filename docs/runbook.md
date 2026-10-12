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

For the Docker path, `task docker:up` pulls and starts the published image
(compose reads `.env`). State is kept in the `kroger-data` volume. To run
local source in Docker, uncomment `build: .` in `docker-compose.yml` and run
`docker compose up -d --build`.

## Common Tasks

See `Taskfile.yml`. Key ones:

```bash
task build         # compile to bin/kroger-mcp
task run           # go run . with .env loaded
task test          # unit + in-memory end-to-end tests
task test:integration  # live tests against the Kroger certification API
task lint          # golangci-lint
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
   with binaries, and then commits `CHANGELOG.md` to `main` in a separate job.

A manual run fails early, before anything is published, if the version isn't
`vX.Y.Z`, the tag already exists, or it wasn't started from `main`.

The first push creates the GHCR package as **private**. Either make it public
(the package page → Package settings → Change visibility) or
`docker login ghcr.io` on the host with a PAT that has `read:packages`.

If `main` is protected, the changelog job's push fails. The release and image
are unaffected. Allow `github-actions[bot]` to bypass the protection, or drop
that job.

## Runtime State

`$KROGER_MCP_DATA_DIR/state.json` (`/data/state.json` in the container) holds the
linked shopper's access and refresh tokens and the preferred store. It is
written with mode 0600. Back it up if you don't want to relink; delete it to
unlink.

## Troubleshooting

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
