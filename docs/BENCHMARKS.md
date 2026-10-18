# Benchmarks

Measured on 18 October 2026 on an Apple M-series laptop (12 cores). Re-run with `go run ./bench -lines 30000`.

## Synthetic corpus: precision and recall

`bench/main.go` builds a corpus in memory: one line in three holds a generated, valid identifier (half with a keyword such as `cnp:` or `iban:`, half under a neutral name); the rest is the noise real repositories are full of: IDs, Unix timestamps, commit hashes, versions, amounts, references and long trace numbers. Every value is generated and belongs to nobody.

"Pattern-only" runs the same candidate patterns without validation, which is roughly what a regex-based scanner does.

    Corpus: 30000 lines (10208 with a value, 19792 without), 0.8 MB, seed 1
    Scan time: 263ms (114209 lines/s, one core)

| Detector | Values | Recall (medium) | Precision (medium) | Recall (low) | Precision (low) | Pattern-only precision |
|---|---|---|---|---|---|---|
| `bg-egn` | 809 | 50.2% | 99.8% | 100.0% | 93.2% | 18.2% |
| `bg-lnch` | 779 | 48.5% | 98.4% | 99.5% | 72.4% | 86.8% |
| `card` | 838 | 100.0% | 88.4% | 100.0% | 88.4% | 18.8% |
| `cz-birth-number` | 774 | 51.2% | 99.5% | 92.9% | 88.5% | 35.0% |
| `email` | 776 | 51.7% | 100.0% | 100.0% | 100.0% | 100.0% |
| `eu-phone` | 796 | 51.6% | 99.8% | 100.0% | 95.4% | 96.0% |
| `eu-vat` | 780 | 50.3% | 100.0% | 100.0% | 100.0% | n/a |
| `hr-oib` | 757 | 48.1% | 99.5% | 99.9% | 92.3% | 52.0% |
| `hu-taj` | 791 | 50.7% | 99.8% | 95.8% | 95.8% | 99.0% |
| `hu-tax-id` | 775 | 53.8% | 100.0% | 94.2% | 99.5% | 99.3% |
| `iban` | 783 | 100.0% | 100.0% | 100.0% | 100.0% | 45.8% |
| `pl-pesel` | 801 | 49.4% | 99.7% | 95.1% | 99.3% | 96.3% |
| `ro-cnp` | 749 | 99.3% | 100.0% | 99.3% | 100.0% | 99.5% |
| **All** | 10208 | 62.0% | 98.1% | 98.2% | 93.5% | 42.3% |

What the numbers say:

- **Validation is what keeps the output usable.** Across all detectors, precision is 98.1% at the default level against 42.3% for pattern-only matching.
- **Low-base detectors trade recall for quiet output.** Ten-digit IDs (EGN, LNCh, HU tax ID), PESEL, OIB, VAT numbers and contact data start at low confidence, so at the default `--min-confidence medium` they are reported only with a nearby keyword (about half the corpus). At `--min-confidence low` recall is 92–100% with precision still 72–100%.
- **Cards are the noisiest identifier.** A random 16-digit number with a Visa or Mastercard prefix passes the Luhn check one time in ten, so long trace and order numbers produce false positives (88% precision here, where the corpus is deliberately full of them). IBANs and CNPs carry enough structure to stay near 100%.

## Real repository: speed and false positives

A shallow clone of [golang/go](https://github.com/golang/go) (231 MB), deleted after the run:

| | |
|---|---|
| Files scanned | 15,321 (787 skipped: binary or over 5 MiB) |
| Wall time | 6.2 s with 12 workers |
| Peak memory | 32 MB |
| Findings at the default level | 6, all email addresses in LICENSE, CONTRIBUTORS and source comments |

The first run on this repository found 34. The run led to three engine fixes in the same pull request: digits that belong to a decimal number (`7.6971174701310497`) are no longer candidates, a leading minus sign counts as part of a number, and a value in a line holding three or more long numbers (tables of seeds or constants) drops one confidence level. Visa is matched at 13 and 16 digits only; 19-digit Visa numbers are rare and random 19-digit constants are common.

## Not compared yet

Microsoft Presidio was not run: it has no recognisers for CNP, EGN, TAJ, PESEL, OIB or birth numbers, so a fair comparison is limited to IBANs, cards, emails and phones and needs its NLP models installed. Tracked in #91.
