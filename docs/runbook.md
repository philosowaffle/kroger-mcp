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
4. `task run` to start on `:8080`, with state in `./data/`.
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
task lint          # golangci-lint
task docker:build  # build image
task docker:up     # docker compose up -d (published image)
task docker:down   # docker compose down
```

## Releasing

1. Merge to `main` with Conventional Commit titles, since git-cliff builds the
   notes from them.
2. `git tag vX.Y.Z && git push origin vX.Y.Z`, or publish a release with a new
   tag in the GitHub UI.
3. The `Release` workflow pushes `ghcr.io/philosowaffle/kroger-mcp` (`X.Y.Z`,
   `X.Y`, `latest`) for amd64 and arm64, creates the GitHub release with
   binaries, and then commits `CHANGELOG.md` to `main` in a separate job.

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
`chain` has to match Kroger's own chain code exactly. Search without it, then
reuse the `chain` value from the results.
