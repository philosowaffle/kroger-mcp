## How to apply

Work this issue on one branch and open one PR. The template's files at the target commit
are at `https://github.com/{{TEMPLATE_REPO}}/tree/{{SHA}}`.

1. **owned:** make this repo's copy identical to the template's at `{{SHA}}`. Don't keep
   local edits; where this repo has to differ, say so in CLAUDE.md under
   Project-Specific Notes instead.
2. **seeded:** this repo owns these files. Apply the intent of the template's change to
   this repo's version, keeping everything repo-specific.
   - An `adopt` row means the file has never been synced: bring it into the template's
     shape. For `CLAUDE.md`, that is the `@docs/process.md` import at the top, then
     Project Overview and Project-Specific Notes. Drop the Standards Version table, the
     Standards deviations paragraph and every section `docs/process.md` now covers, but
     keep every repo-specific rule. For `pr-checks.yml`, keep the repo's own jobs and
     steps (integration tests, extra tools) and drop the `tmp/` check.
   - For server files the repo already has (`main.go`, `Taskfile.yml`, `Dockerfile`,
     compose, `release.yml`), keep this repo's version and its extra services, tools
     and steps. Take only the shared behaviour the template's version adds, and the
     calls the owned packages now need (`auth.ConfigFromEnv(server.Name,
     server.EnvPrefix)`, `logging.Setup(server.EnvPrefix)`, and in `main.go`
     `telemetry.Setup(ctx, server.Name, server.Version)` with its shutdown,
     `telemetry.Instrument(srv)` on the MCP server and `telemetry.HTTPHandler` around
     the MCP endpoint), adding `Name` and `EnvPrefix` to `internal/server` if they're
     missing. Keep the repo's env variable
     names: `EnvPrefix` must match them.
3. **retired:** delete the path, and anything that only pointed to it.
4. Write `{{SHA}}` to `.template-version` (just the SHA and a newline).
5. Run `go mod tidy` (owned packages may import new modules), then `task ci`, and fix
   what breaks.
6. Open a PR titled `chore: sync with go-mcp-template`, labelled `chore`, that closes
   this issue. In its body, list anything you didn't apply and why.

A PR that changes `.github/workflows/` needs the repo owner to merge it.
