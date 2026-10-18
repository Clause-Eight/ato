# Benchmarks

Measured on 19 October 2026 on an Apple M-series laptop (12 cores). Re-run with `go run ./bench -lines 30000`.

## Synthetic corpus: precision and recall

`bench/main.go` builds a corpus in memory: one line in three holds a generated, valid identifier (half with a keyword such as `cnp:` or `iban:`, half under a neutral name); the rest is the noise real repositories are full of: IDs, Unix timestamps, commit hashes, versions, amounts, references and long trace numbers. Every value is generated and belongs to nobody.

"Pattern-only" runs the same candidate patterns without validation, which is roughly what a regex-based scanner does.

    Corpus: 30000 lines (10036 with a value, 19964 without), 0.8 MB, seed 1
    Scan time: 212ms (141462 lines/s, one core)

| Detector | Values | Recall (medium) | Precision (medium) | False positives (medium) | Recall (low) | Precision (low) | Pattern-only precision |
|---|---|---|---|---|---|---|---|
| `advertising-id` | 382 | 50.8% | 100.0% | 0 | 50.8% | 100.0% | 100.0% |
| `bg-egn` | 399 | 46.6% | 100.0% | 0 | 100.0% | 91.9% | 12.6% |
| `bg-lnch` | 323 | 55.7% | 97.3% | 5 | 99.4% | 56.2% | 91.4% |
| `card` | 373 | 91.4% | 95.0% | 18 | 100.0% | 77.1% | 9.6% |
| `cz-birth-number` | 336 | 100.0% | 99.7% | 1 | 100.0% | 81.0% | 35.5% |
| `de-id-card` | 354 | 48.9% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `de-pension` | 381 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 99.7% |
| `de-tax-id` | 373 | 67.8% | 100.0% | 0 | 100.0% | 98.4% | 39.6% |
| `email` | 349 | 53.6% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `es-dni` | 377 | 54.9% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `es-nie` | 337 | 52.8% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `es-nss` | 346 | 86.7% | 100.0% | 0 | 100.0% | 99.7% | 54.1% |
| `eu-phone` | 350 | 84.9% | 100.0% | 0 | 100.0% | 90.9% | 90.3% |
| `eu-vat` | 356 | 52.5% | 100.0% | 0 | 100.0% | 100.0% | 98.3% |
| `fr-nir` | 366 | 78.1% | 99.7% | 1 | 100.0% | 99.7% | 99.3% |
| `fr-spi` | 384 | 75.3% | 100.0% | 0 | 100.0% | 100.0% | 98.3% |
| `hr-oib` | 381 | 50.1% | 99.5% | 1 | 98.4% | 90.1% | 83.7% |
| `hu-taj` | 335 | 51.6% | 100.0% | 0 | 97.6% | 89.3% | 96.6% |
| `hu-tax-id` | 323 | 55.1% | 100.0% | 0 | 95.0% | 99.4% | 100.0% |
| `iban` | 327 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 43.7% |
| `imei` | 360 | 85.0% | 100.0% | 0 | 100.0% | 98.1% | 98.4% |
| `ip-address` | 395 | 51.6% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `it-fiscal-code` | 365 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `mac-address` | 361 | 52.9% | 100.0% | 0 | 52.9% | 100.0% | 100.0% |
| `mrz` | 358 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `nl-bsn` | 321 | 64.8% | 100.0% | 0 | 93.1% | 93.7% | 99.5% |
| `pl-pesel` | 378 | 56.3% | 99.5% | 1 | 95.5% | 99.2% | 98.6% |
| `ro-cnp` | 346 | 98.6% | 99.7% | 1 | 98.6% | 99.7% | 59.1% |
| **All** | 10036 | 70.0% | 99.6% | 28 | 95.7% | 93.8% | 48.9% |

What the numbers say:

- **Validation is what keeps the output usable.** Across all detectors, precision is 99.6% at the default level against 48.9% for pattern-only matching.
- **Low-base detectors trade recall for quiet output.** Bare numbers with one check digit (EGN, LNCh, PESEL, OIB, BSN, the Nordic and Baltic codes), VAT numbers and contact data start at low confidence, so at the default `--min-confidence medium` they are reported only with a nearby keyword or in their usual printed format. Detectors marked "only with a keyword" are never reported without one.
- **Many national numbers share a shape.** An eleven-digit number can be a valid PESEL, OIB, German tax ID and Estonian code at once. Each value is reported once, preferring the detector whose own name is nearby, so at `--min-confidence low` some values are counted under another country and precision there is lower.
- **Cards are the noisiest identifier.** A random 16-digit number with a Visa or Mastercard prefix passes the Luhn check one time in ten, so long trace and order numbers look like cards. A nearby word that usually labels other numbers (trace, order, invoice, transaction) lowers a card one step unless a card keyword is also there, which keeps card precision at the default level near 95%.

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
