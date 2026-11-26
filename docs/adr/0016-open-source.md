# 16. ATO is open source

Date: 2026-11-26 · Status: accepted

Supersedes [ADR 3](0003-proprietary.md).

## Decision

- ATO is licensed under the Apache License 2.0 from this commit on, as pii-eu was up to v0.2.0.
- The repository is public. Outside contributions are accepted under the same licence.
- Installing no longer needs repository access or `GOPRIVATE`.

## Consequences

- The offline license check from [ADR 15](0015-licensing.md) is removed: no `ato license` command, no
  `ATO_LICENSE` variable, no evaluation notice, and no `cmd/ato-license`. It never shipped in a release.
- Versions released while the repository was private keep the licence they were released under.
