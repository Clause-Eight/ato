# pii-eu

pii-eu finds EU personal data (national IDs, IBANs, cards, VAT numbers, contact data) in code, fixtures and logs, and confirms each match with the identifier's own check digits. Go, one static binary. Layers and import rules: docs/adr/0002-layers.md.

## Rules

- Clean room: synthetic test data only; nothing from an employer, client or real person. See CONTRIBUTING.md.
- Commit identity, set locally: `user.name "Kiwi"`, `user.email "53535321+Kiwi-toor@users.noreply.github.com"`. Check `git config --local user.email` before committing.
- Found values never appear in reports, logs, fingerprints, test names or commit messages. Reporters only see masked values.
- Conventional commits, branches `<type>/<topic>`, no Claude attribution, merge commits.
- The repository stays private until the owner says otherwise.

## Commands

```bash
gofmt -l . && go vet ./... && go test ./...
go run ./cmd/pii-eu scan .
```
