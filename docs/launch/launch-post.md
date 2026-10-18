# Draft: launch post

Status: ready to post with the v0.2.0 release.

---

## pii-eu: find EU personal data in your code, confirmed by check digits

Personal data ends up in places nobody meant it to: a fixture copied from production, an IBAN in a test, a card number in a log line, a national ID in a seed script. Once it is in git history it is copied to every clone, CI cache and laptop.

Most secret scanners look for keys and tokens. Most personal-data scanners are built around US formats, need a Python service and NLP models, and match numbers by shape, which buries real findings under order IDs and timestamps.

pii-eu is a small, free, open-source CLI for European identifiers. It finds Romanian CNPs, Bulgarian EGNs, Hungarian TAJ and tax IDs, Polish PESELs, Czech and Slovak birth numbers, Croatian OIBs, IBANs, payment cards, EU VAT numbers, emails and phone numbers, and it **confirms every match with the identifier's own rules**: the CNP's county code and weighted check digit, IBAN mod 97, the Luhn check, the date hidden in a PESEL.

On a synthetic benchmark that mixes generated identifiers with the noise real repositories are full of, that validation takes precision from 42% (pattern matching alone) to 98%. On the Go repository (15,000 files) it runs in about six seconds and reports six findings, all real email addresses.

It never prints what it finds. Reports show masked values (`••••••••••••7892`), and baselines and SARIF fingerprints are derived from the file and the line with values removed, so the scanner does not become one more place the data leaks to.

```bash
brew install clause-eight/tap/pii-eu
# or: go install github.com/Clause-Eight/pii-eu/cmd/pii-eu@latest
pii-eu scan .
```

It runs as a pre-commit hook, a GitHub Action with pull-request annotations, a GitLab Code Quality job or a Bitbucket step, and it reads log streams from stdin.

Adding a country is one Go file and its tests, and contributions are welcome. All test data is synthetic, and the project accepts no real personal data in issues, tests or examples.

Apache 2.0. github.com/Clause-Eight/pii-eu
