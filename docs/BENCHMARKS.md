# Benchmarks

Measured on 18 October 2026 on an Apple M-series laptop (12 cores). Re-run with `go run ./bench -lines 30000`.

## Synthetic corpus: precision and recall

`bench/main.go` builds a corpus in memory: one line in three holds a generated, valid identifier (half with a keyword such as `cnp:` or `iban:`, half under a neutral name); the rest is the noise real repositories are full of: IDs, Unix timestamps, commit hashes, versions, amounts, references and long trace numbers. Every value is generated and belongs to nobody.

"Pattern-only" runs the same candidate patterns without validation, which is roughly what a regex-based scanner does.

    Corpus: 30000 lines (10173 with a value, 19827 without), 0.8 MB, seed 1
    Scan time: 272ms (110447 lines/s, one core)

| Detector | Values | Recall (medium) | Precision (medium) | Recall (low) | Precision (low) | Pattern-only precision |
|---|---|---|---|---|---|---|
| `bg-egn` | 803 | 50.1% | 99.8% | 100.0% | 92.8% | 18.1% |
| `bg-lnch` | 783 | 48.4% | 98.4% | 99.4% | 72.2% | 87.0% |
| `card` | 844 | 100.0% | 88.6% | 100.0% | 88.6% | 18.8% |
| `cz-birth-number` | 757 | 50.5% | 99.7% | 92.9% | 89.2% | 34.8% |
| `email` | 785 | 51.6% | 100.0% | 100.0% | 100.0% | 100.0% |
| `eu-phone` | 800 | 51.7% | 99.8% | 100.0% | 95.2% | 95.7% |
| `eu-vat` | 780 | 50.1% | 100.0% | 100.0% | 100.0% | n/a |
| `hr-oib` | 763 | 47.7% | 99.7% | 99.9% | 92.7% | 52.8% |
| `hu-taj` | 772 | 50.8% | 99.7% | 96.1% | 95.5% | 99.0% |
| `hu-tax-id` | 786 | 54.1% | 100.0% | 94.0% | 99.5% | 99.3% |
| `iban` | 776 | 100.0% | 100.0% | 100.0% | 100.0% | 45.5% |
| `pl-pesel` | 780 | 49.5% | 99.7% | 95.5% | 99.6% | 95.9% |
| `ro-cnp` | 744 | 99.2% | 100.0% | 99.2% | 100.0% | 99.5% |
| **All** | 10173 | 61.9% | 98.1% | 98.3% | 93.5% | 42.2% |

What the numbers say:

- **Validation is what keeps the output usable.** Across all detectors, precision is 98.1% at the default level against 42.2% for pattern-only matching.
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

## Compared with Microsoft Presidio

[Presidio](https://github.com/microsoft/presidio) (presidio-analyzer 2.2, spaCy `en_core_web_sm`) on the same corpus, for the five identifiers both tools cover. Presidio's phone recognizer was given the RO, BG, HU and PL regions and its PESEL recognizer was enabled for English text, so it is not penalised for its defaults. Run it with `bench/presidio_compare.py`.

| Identifier | pii-eu recall / precision (default) | pii-eu recall / precision (low) | Presidio recall / precision (any score) | Presidio recall / precision (score ≥ 0.5) |
|---|---|---|---|---|
| `card` | 100.0% / 88.6% | 100.0% / 88.6% | 100.0% / 82.1% | 100.0% / 82.1% |
| `iban` | 100.0% / 100.0% | 100.0% / 100.0% | 100.0% / 100.0% | 100.0% / 100.0% |
| `email` | 51.6% / 100.0% | 100.0% / 100.0% | 100.0% / 100.0% | 100.0% / 100.0% |
| `eu-phone` | 51.7% / 99.8% | 100.0% / 95.2% | 87.6% / 40.4% | 44.5% / 100.0% |
| `pl-pesel` | 49.5% / 99.7% | 95.5% / 99.6% | 100.0% / 64.5% | 100.0% / 64.5% |

- **Speed:** Presidio scanned the 30,000 lines in 57.9 s (518 lines/s); pii-eu takes about 0.3 s (110,447 lines/s), roughly 200 times faster on one core, without an NLP model.
- **Coverage:** Presidio has no recognisers for CNP, EGN, LNCh, TAJ, Hungarian tax ID, Czech and Slovak birth numbers, Croatian OIB or most EU VAT numbers (it has German and Italian ones). Those are 8 of pii-eu's 13 detectors.
- **Where Presidio is ahead:** it reports every email and phone number with no keyword needed. pii-eu reports them only next to a keyword at the default level, by design; at `--min-confidence low` it finds all of them.
- **Where pii-eu is ahead:** PESEL precision (99.7% against 64.5%: Presidio checks the check digit and a rough month and day pattern, but not real calendar dates, and its pattern has no boundaries, so it can match inside longer numbers), phone precision at Presidio's lower scores, and card precision (88.6% against 82.1%).
- **What it taught us:** Presidio checks each country's account-number format inside the IBAN. An earlier version of this corpus had Bulgarian IBANs with an all-digit bank code; they pass mod 97, but the real format starts with four letters. Presidio rightly rejected them and pii-eu did not. The corpus is fixed, and checking the national account format is tracked as an improvement to pii-eu.
