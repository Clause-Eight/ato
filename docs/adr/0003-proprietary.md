# 3. pii-eu is proprietary

Date: 2026-10-19 · Status: accepted

pii-eu is called ATO since 2.0.0; see [ADR 12](0012-rename-to-ato.md).

## Context

pii-eu was published under Apache-2.0 up to v0.2.0, as a free tool next to Clause Eight. Clause Eight itself
became proprietary on 2026-10-17, and pii-eu's detectors, schema reading and data inventory are now part of
what Clause Eight sells.

## Decision

- From v0.3.0, pii-eu is all rights reserved, under the same licence as Clause Eight.
- The repository and the Homebrew tap are private. Distribution is to Clause Eight customers only.
- v0.2.0 and earlier stay under Apache-2.0 for anyone who already has them; that cannot be withdrawn.

## Consequences

- No outside contributions; CONTRIBUTING.md is the internal guide for adding detectors.
- CI minutes count against the organisation's quota, so CI runs once per pull request and builds the other
  platforms only for releases.
- Installing needs repository access: `GOPRIVATE=github.com/Clause-Eight/*` for `go install`, or a release
  binary downloaded with a token.
