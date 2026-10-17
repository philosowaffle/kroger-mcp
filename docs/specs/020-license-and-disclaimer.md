# Spec: GPL-3.0 LICENSE and Kroger non-affiliation disclaimer

**Issue:** #20
**Status:** Complete

## Problem

The repo has no LICENSE, so by default the code is "all rights reserved" and nobody can
legally use, fork or contribute to it. The project also uses Kroger's name and its banner
names (King Soopers, Ralphs, ...), so it needs to say clearly that it's unofficial.

## Acceptance Criteria

- [x] `LICENSE` holds the unmodified GPL-3.0 text, so GitHub detects the license.
- [x] The README has a non-affiliation disclaimer that also says Kroger API use falls
      under Kroger's developer terms, which each self-hoster accepts with their own app.
- [x] The Kroger OAuth callback page carries the disclaimer.
- [x] The image carries `org.opencontainers.image.licenses`, in both release images and
      local `docker build`s.
- [x] The README shows a license badge.

## Notes

- `LICENSE` is the verbatim GPL-3.0 text from `github/choosealicense.com`, which is
  what GitHub's license detection (licensee) matches against.
- SPDX id is `GPL-3.0-only` (Dockerfile `LABEL` and the release workflow's
  `docker/metadata-action` labels). Switching to `-or-later`, or to AGPL-3.0 (the issue
  raises it because this is a network service), means changing `LICENSE`, those two
  labels and the README badge.
- The callback page footer is checked in `TestCallbackHandler/unknown_state`.
- The Pages site (#22) doesn't exist yet. Its disclaimer is tracked there.
