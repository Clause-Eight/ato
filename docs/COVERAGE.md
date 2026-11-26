# Coverage by country

National identifiers ATO confirms, per country. Generated from the detector registry; a test fails when this table is out of date. How each one is checked: [DETECTORS.md](DETECTORS.md).

<!-- coverage:start -->
| Country | Personal number | Tax ID | Social security or health | ID card or passport |
|---|---|---|---|---|
| AT | – | `at-tax-number` | `at-social-insurance` | – |
| BE | `be-national-register` | – | – | `be-id-card` |
| BG | `bg-egn`, `bg-lnch` | – | – | `bg-id-card` |
| CY | – | `cy-tic` | – | `cy-id-card` |
| CZ | `cz-birth-number` | – | – | `cz-bank-account` |
| DE | – | `de-stnr`, `de-tax-id` | `de-kvnr`, `de-pension` | `de-id-card` |
| DK | `dk-cpr`* | – | – | – |
| EE | `ee-isikukood` | – | – | – |
| ES | `es-dni`, `es-nie` | – | `es-nss` | `es-cadastral-reference`, `es-ccc`, `es-cups` |
| FI | `fi-hetu` | – | – | `fi-veronumero` |
| FR | – | `fr-spi` | `fr-nir` | `fr-id-card` |
| GR | – | `gr-afm` | `gr-amka` | `gr-id-card` |
| HR | `hr-oib` | – | `hr-mbo` | – |
| HU | `hu-personal-id` | `hu-tax-id` | `hu-taj` | `hu-id-card` |
| IE | `ie-ppsn` | – | – | `ie-eircode` |
| IT | – | `it-fiscal-code` | – | – |
| LT | `lt-asmens-kodas` | – | – | – |
| LU | `lu-national-id` | – | – | – |
| LV | `lv-personal-code`, `lv-personal-code-new` | – | – | – |
| MT | – | – | – | `mt-id-card` |
| NL | `nl-bsn` | – | – | `nl-id-card`, `nl-onderwijsnummer` |
| PL | `pl-pesel` | `pl-nip` | – | `pl-id-card`, `pl-nrb`, `pl-regon` |
| PT | – | `pt-nif` | `pt-niss` | `pt-citizen-card` |
| RO | `ro-cnp` | – | – | `ro-cei`, `ro-id-card` |
| SE | `se-personnummer` | – | – | – |
| SI | `si-emso` | `si-tax-number` | – | – |
| SK | `cz-birth-number` | – | – | – |
| CH | – | – | `ch-ahv`* | `ch-qr-reference`* |
| GB | – | `gb-utr`* | `gb-nhs`*, `gb-nino`* | `gb-upn`* |
| IS | `is-kennitala`* | – | – | – |
| LI | – | – | – | `li-peid`* |
| NO | `no-fnr`* | – | – | `no-kontonr`* |

* off by default; on when the country is selected with `--countries`. IBANs, cards, VAT numbers, contact data and the other cross-border detectors cover every country.
<!-- coverage:end -->
