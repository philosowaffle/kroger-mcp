# Spec: GPL-3.0 LICENSE and Kroger non-affiliation disclaimer

**Issue:** #20
**Status:** In Progress

## Problem

The repo has no LICENSE, so by default the code is "all rights reserved" and nobody can
legally use, fork or contribute to it. The project also uses Kroger's name and its banner
names (King Soopers, Ralphs, ...), so it needs to say clearly that it's unofficial.

## Acceptance Criteria

- [ ] `LICENSE` holds the unmodified GPL-3.0 text, so GitHub detects the license.
- [ ] The README has a non-affiliation disclaimer that also says Kroger API use falls
      under Kroger's developer terms, which each self-hoster accepts with their own app.
- [ ] The Kroger OAuth callback page carries the disclaimer.
- [ ] The image carries `org.opencontainers.image.licenses`, in both release images and
      local `docker build`s.
- [ ] The README shows a license badge.

## Notes

- The Pages site (#22) doesn't exist yet. Its disclaimer is tracked there.
