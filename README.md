# pii-eu

Find EU personal data in code, test fixtures and logs. Every match is confirmed with the identifier's own rules (check digits, dates, lengths), so findings are few and real.

Work in progress. The plan is on the [project board](https://github.com/orgs/Clause-Eight/projects/1).

```bash
go run ./cmd/pii-eu scan .
```

Exit codes: `0` nothing found, `1` findings, `2` usage or settings error, `3` some files could not be read.

Licensed under the Apache License 2.0.
