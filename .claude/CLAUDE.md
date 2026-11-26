# ATO

ATO (called pii-eu before 2.0) finds EU personal data (national IDs, IBANs, cards, VAT numbers, contact data) in code, fixtures and logs, and confirms each match with the identifier's own check digits. Go, one static binary. Layers and import rules: docs/adr/0002-layers.md.

## Rules

- Clean room: synthetic test data only; nothing from an employer, client or real person. See CONTRIBUTING.md.
- Commit identity, set locally: `user.name "Kiwi"`, `user.email "53535321+Kiwi-toor@users.noreply.github.com"`. Check `git config --local user.email` before committing.
- Found values never appear in reports, logs, fingerprints, test names or commit messages. Reporters only see masked values.
- Conventional commits, branches `<type>/<topic>`, no Claude attribution, merge commits.
- ATO is open source under Apache-2.0 since 2026-11-26 (docs/adr/0016-open-source.md); the repository is public. No internal links, no real data, no employer material.
- CI minutes are limited: verify locally, push only finished branches, one pull request per milestone.
- Releases: tag `vX.Y.Z` on main; the release workflow builds, signs and publishes.

## Commands

```bash
gofmt -l . && go vet ./... && go test ./...
go run ./cmd/ato scan .
```
