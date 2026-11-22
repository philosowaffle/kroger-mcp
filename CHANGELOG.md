# Changelog
## [1.1.0] - 2026-11-22

### CI/CD
- release automatically when main passes PR Checks after a merge (#62)


### Features
- redesign the Pages site with a grocery receipt look (#63)


## [1.0.0] - 2026-11-22

### Bug Fixes
- reject ID tokens and harden the HTTP surface (#34)
- fail closed without MCP auth, stricter lint and build-time version (#37)
- serve MCP statelessly so calls with protocolVersion _meta work (#49)
- keep Kroger's error code and name the failing request in API errors (#50)


### CI/CD
- publish multi-arch image to GHCR on release (#16)
- add manual release trigger and publish main image on merge (#17)
- add GPL-3.0 license and harden the CI supply chain (#39)
- bump the github-actions group with 5 updates (#42)
- bump the github-actions group across 1 directory with 6 updates (#51)
- add go-mcp-template sync workflow (#54)
- drop the duplicate CodeQL workflow (#59)


### Documentation
- prune stale docs and standards references (#41)
- add SECURITY.md, CONTRIBUTING.md and CODE_OF_CONDUCT (#43)
- rewrite README for a public audience (#53)


### Features
- pivot to a dockerized Kroger MCP server (#15)
- cover the full Kroger public API with lite/full detail levels (#18)
- handle Kroger rate limits and fix brand-only product search (#36)
- add GitHub Pages landing site (#55)


