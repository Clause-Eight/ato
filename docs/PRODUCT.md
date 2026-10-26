# pii-eu as a product

pii-eu ships inside Clause Eight and is also sold on its own. This page says who it is for, what it is up
against and how it is packaged. Pricing below is a set of hypotheses to test with buyers, not a price list.

## The problem

EU companies have to know where personal data lives (GDPR Art. 30), keep special categories under tighter
rules (Art. 9), erase on request (Art. 17) and keep data no longer than needed (Art. 5(1)(e)). The record of
processing is usually a spreadsheet filled in by hand, out of date the week after it is written, and nobody can
show where it came from.

Developers leak personal data into fixtures, logs and analytics without meaning to. Generic scanners either
miss EU identifiers or flag every ten-digit number.

## Who buys

| Buyer | Pain | What they get |
|---|---|---|
| DPO or privacy lead at an EU SaaS or fintech (50–1000 staff) | Art. 30 record and DPIAs maintained by hand | A data map drafted from code, re-checked on every change, with a source for every line |
| Security or platform team | Personal data in logs, dumps, images and buckets | Findings in CI, scans of exports and container images, Terraform checks |
| Consultants and auditors (ISO 27001, GDPR) | Evidence collection is slow | One self-contained report per client repository |

First market: CEE and the rest of the EU where national identifiers matter (RO, BG, HU, PL, CZ, the Baltics),
the same wedge as Clause Eight.

## What makes it different

- **EU identifiers confirmed by their own rules.** Check digits, dates and region codes for every member state,
  keywords in all 24 EU languages. Findings are few and real.
- **Code to evidence.** The inventory, data map, erasure and retention checks cite file and line, so a DPO can
  check every claim.
- **Runs where the data is.** One static binary, no network calls, nothing leaves the customer's machines.
- **Infrastructure in the same report.** Terraform stores without encryption, retention or EU residency, linked
  to the personal data they hold.

## Competition

| Tool | What it does | Where pii-eu differs |
|---|---|---|
| Privado | Code scanning for data flows and RoPA; open-source core (Java, Python), commercial platform | EU national identifiers with check digits, 24 languages, data files and images, single binary |
| HoundDog.ai | Privacy code scanner: PII in logs, third parties, RoPA and PIA reports | Same as above; EU-first; offline; Terraform residency and retention |
| Microsoft Presidio | Open-source PII detection library, mostly US identifiers, NLP based | Validated EU identifiers; a product, not a library; see the comparison in docs/BENCHMARKS.md |
| Nightfall, Purview, Macie, Google Sensitive Data Protection | Cloud DLP over SaaS and storage | Works in code and CI before data spreads; no data leaves the machine |

Privado and HoundDog.ai already sell the data map and log-leak ideas. pii-eu should not compete on breadth of
languages; it wins on EU accuracy, evidence quality and running offline.

## Packaging

| Package | Contents | For |
|---|---|---|
| CLI and CI | `scan`, `fix`, `baseline`, CI templates for GitHub, GitLab and Bitbucket, pre-commit | Developers and security teams |
| Privacy evidence | `inventory`, `map`, erasure and retention checks, HTML reports | DPOs and auditors |
| Data scanning | Exports, dumps, archives and container images | Security teams |
| Inside Clause Eight | All of the above as a Clause Eight scanner, results in Clause Eight Cloud | Clause Eight customers |

All packages are one binary. Entitlements are decided by distribution (who gets which build or licence file),
not by switches that a customer could remove.

## Pricing hypotheses to test

- Per repository per year, with a floor: about €300 per repository, minimum €3,000.
- Privacy evidence sold per organisation to DPOs: €6,000–12,000 a year.
- Consultants: per client report, or a partner licence.
- Included in Clause Eight's paid plans.

Test in the Clause Eight buyer conversations before building billing.

## What must be true before selling on its own

1. Employment and IP clearance (Clause Eight milestone M1).
2. Distribution without repository access (ADR 0006).
3. A product page and docs.
4. Terms, DPA and an EU company to sign them, shared with Clause Eight.
