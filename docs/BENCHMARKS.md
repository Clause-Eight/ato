# Benchmarks

Measured on 19 October 2026 on an Apple M-series laptop (12 cores). Re-run with `go run ./bench -lines 30000`.

## Synthetic corpus: precision and recall

`bench/main.go` builds a corpus in memory: one line in three holds a generated, valid identifier (half with a keyword such as `cnp:` or `iban:`, half under a neutral name); the rest is the noise real repositories are full of: IDs, Unix timestamps, commit hashes, versions, amounts, references and long trace numbers. Every value is generated and belongs to nobody.

"Pattern-only" runs the same candidate patterns without validation, which is roughly what a regex-based scanner does.

    Corpus: 30000 lines (10054 with a value, 19946 without), 0.8 MB, seed 1
    Scan time: 398ms (75339 lines/s, one core)

| Detector | Values | Recall (medium) | Precision (medium) | False positives (medium) | Recall (low) | Precision (low) | Pattern-only precision |
|---|---|---|---|---|---|---|---|
| `advertising-id` | 201 | 45.3% | 100.0% | 0 | 45.3% | 100.0% | 100.0% |
| `at-social-insurance` | 188 | 76.6% | 100.0% | 0 | 100.0% | 95.4% | 6.1% |
| `be-national-register` | 200 | 75.0% | 100.0% | 0 | 100.0% | 98.5% | 17.9% |
| `bg-egn` | 185 | 48.6% | 100.0% | 0 | 99.5% | 81.8% | 78.3% |
| `bg-lnch` | 194 | 52.6% | 98.1% | 2 | 100.0% | 42.4% | 89.5% |
| `card` | 200 | 87.5% | 97.8% | 4 | 100.0% | 61.2% | 5.5% |
| `ch-ahv` | 197 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `cy-id-card` | 194 | 46.9% | 100.0% | 0 | 46.9% | 100.0% | 100.0% |
| `cz-birth-number` | 190 | 100.0% | 100.0% | 0 | 100.0% | 63.1% | 21.9% |
| `de-id-card` | 179 | 44.7% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `de-pension` | 167 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `de-tax-id` | 173 | 65.3% | 100.0% | 0 | 100.0% | 95.1% | 93.4% |
| `dk-cpr` | 202 | 53.0% | 97.3% | 3 | 53.0% | 97.3% | 90.5% |
| `ee-isikukood` | 198 | 56.1% | 97.4% | 3 | 99.0% | 67.4% | 95.5% |
| `email` | 184 | 55.4% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `es-dni` | 196 | 46.4% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `es-nie` | 204 | 49.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `es-nss` | 192 | 85.9% | 100.0% | 0 | 100.0% | 100.0% | 36.9% |
| `eu-phone` | 202 | 81.2% | 100.0% | 0 | 100.0% | 84.5% | 82.7% |
| `eu-vat` | 181 | 43.6% | 100.0% | 0 | 100.0% | 100.0% | 92.9% |
| `fi-hetu` | 158 | 100.0% | 97.5% | 4 | 100.0% | 97.5% | 49.4% |
| `fr-nir` | 171 | 73.1% | 100.0% | 0 | 100.0% | 99.4% | 99.3% |
| `fr-spi` | 190 | 74.7% | 100.0% | 0 | 100.0% | 99.5% | 100.0% |
| `gb-nhs` | 179 | 75.4% | 100.0% | 0 | 96.6% | 75.5% | 92.4% |
| `gb-nino` | 183 | 71.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `gr-afm` | 193 | 40.4% | 97.5% | 2 | 90.2% | 77.0% | 96.3% |
| `gr-amka` | 170 | 52.4% | 100.0% | 0 | 99.4% | 94.4% | 95.7% |
| `hr-oib` | 192 | 49.0% | 100.0% | 0 | 96.4% | 79.4% | 96.8% |
| `hu-taj` | 193 | 54.4% | 100.0% | 0 | 92.2% | 79.1% | 82.7% |
| `hu-tax-id` | 194 | 51.5% | 100.0% | 0 | 91.2% | 98.9% | 100.0% |
| `iban` | 195 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 45.7% |
| `ie-ppsn` | 192 | 55.2% | 100.0% | 0 | 100.0% | 98.0% | 69.1% |
| `imei` | 179 | 84.4% | 99.3% | 1 | 100.0% | 96.8% | 98.7% |
| `ip-address` | 174 | 44.8% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `is-kennitala` | 165 | 73.3% | 99.2% | 1 | 89.7% | 98.0% | 93.7% |
| `it-fiscal-code` | 202 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `lt-asmens-kodas` | 192 | 51.0% | 99.0% | 1 | 51.0% | 99.0% | 95.1% |
| `lu-national-id` | 187 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 93.0% |
| `lv-personal-code` | 199 | 86.4% | 100.0% | 0 | 97.5% | 100.0% | 55.3% |
| `lv-personal-code-new` | 174 | 54.6% | 100.0% | 0 | 54.6% | 100.0% | n/a |
| `mac-address` | 176 | 48.9% | 100.0% | 0 | 48.9% | 100.0% | 100.0% |
| `mrz` | 156 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `mt-id-card` | 174 | 51.1% | 100.0% | 0 | 51.1% | 100.0% | 100.0% |
| `nl-bsn` | 186 | 55.4% | 100.0% | 0 | 89.2% | 92.2% | 100.0% |
| `no-fnr` | 203 | 63.1% | 100.0% | 0 | 96.1% | 100.0% | 96.9% |
| `pl-id-card` | 197 | 52.8% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `pl-pesel` | 178 | 48.3% | 100.0% | 0 | 92.7% | 94.3% | 97.6% |
| `pl-regon` | 198 | 52.5% | 100.0% | 0 | 52.5% | 100.0% | 96.3% |
| `pt-citizen-card` | 177 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 100.0% |
| `pt-nif` | 166 | 54.2% | 100.0% | 0 | 83.7% | 95.9% | 100.0% |
| `ro-cnp` | 177 | 99.4% | 99.4% | 1 | 99.4% | 99.4% | 27.5% |
| `ro-id-card` | 171 | 46.2% | 100.0% | 0 | 46.2% | 100.0% | 100.0% |
| `se-personnummer` | 187 | 75.4% | 98.6% | 2 | 93.6% | 90.2% | 88.7% |
| `si-emso` | 199 | 100.0% | 100.0% | 0 | 100.0% | 100.0% | 73.6% |
| **All** | 10054 | 67.5% | 99.6% | 24 | 89.9% | 90.2% | 43.5% |

What the numbers say:

- **Validation is what keeps the output usable.** Across all detectors, precision is 99.6% at the default level against 43.5% for pattern-only matching.
- **Low-base detectors trade recall for quiet output.** Bare numbers with one check digit (EGN, LNCh, PESEL, OIB, BSN, the Nordic and Baltic codes), VAT numbers and contact data start at low confidence, so at the default `--min-confidence medium` they are reported only with a nearby keyword or in their usual printed format. Detectors marked "only with a keyword" are never reported without one.
- **Many national numbers share a shape.** An eleven-digit number can be a valid PESEL, OIB, German tax ID and Estonian code at once. Each value is reported once, preferring the detector whose own name is nearby, so at `--min-confidence low` some values are counted under another country and precision there is lower.
- **Cards are the noisiest identifier.** A random 16-digit number with a Visa or Mastercard prefix passes the Luhn check one time in ten, so long trace and order numbers look like cards. A nearby word that usually labels other numbers (trace, order, invoice, transaction) lowers a card one step unless a card keyword is also there, which keeps card precision at the default level near 95%.

## Real repository: speed and false positives

Go's own source tree (`$(go env GOROOT)/src`, Go 1.26, 150 MB, 10,720 files), measured on 19 October 2026 with the same machine:

| | v0.2.0 (13 detectors) | v0.4.0 (54 detectors) |
|---|---|---|
| Wall time | 13.4 s | 15.4 s |
| Findings at the default level | 5, all email addresses | the same 5 |

Four times the detectors cost 15% more time on real code, because the prefilter skips most detectors on most lines. Synthetic, digit-heavy text slows down more.

The earlier measurement, for v0.2.0:

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
