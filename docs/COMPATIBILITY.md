# Compatibility

ATO, Clause Eight's EU personal-data scanner, follows semantic versioning from 1.0.0. It was called pii-eu
before 2.0.0 ([ADR 12](adr/0012-rename-to-ato.md)); what the rename changed, and which 1.x names still work, is
in [Changes in 2.0](#changes-in-20). This page says what a 2.x release keeps working, and how a change is
announced before it breaks anything.

## Stable in every 2.x release

| Interface | Promise |
|---|---|
| Commands | `ato` with `scan`, `baseline`, `inventory`, `map`, `fix`, `detectors` and `version` keeps its name and meaning. |
| Flags | Every flag listed by `ato help` keeps its name, its meaning and its default. |
| Exit codes | `0` nothing found, `1` findings, `2` usage or settings error, `3` the scan is incomplete: some files could not be read, or were not read in full. `ato fix` exits `0` when it changed files or had nothing to do. |
| Settings file | `.ato.toml` keys keep their names and meaning. An unknown key stays an error. |
| JSON report | `schemas/findings.schema.json`: fields listed there are never removed, renamed or given a new type. `schema_version` stays `2.x`; `tool.name` is `ato`. |
| Inventory | `schemas/inventory.schema.json` and the CSV columns, under the same rule. |
| Data map | `schemas/map.schema.json`, under the same rule; `map --diff` exits `1` when something is new, and reads maps saved by 1.x and 2.x. |
| SARIF | Rule IDs (`<detector>`, `special-category/<category>`, `log-sink/<sink>`, the Terraform checks `tf-unencrypted`, `tf-no-retention`, `tf-non-eea-region`, `tf-public`, and `no-erasure-path`, `no-retention`), the driver name `ATO`, and the `ato/v1` partial fingerprint. The deprecated `piiEu/v1` fingerprint is written with the same value in every 2.x release. |
| GitLab Code Quality | `check_name` is `ato/<detector>`; `fingerprint` follows the fingerprint rule below. |
| Detector names | A detector's name never changes and is never reused for a different identifier. The same holds for `special-category`, `log-sink`, the `tf-*` checks, `no-erasure-path` and `no-retention`. |
| Fingerprints and baselines | A finding's fingerprint stays the same for the same detector, path and line text, so a committed baseline keeps working, including one written by 1.x. |
| Masking | No output format ever prints a found value, except text output with `--show-values` on screen. `ato fix --dry-run` masks found values and shows only the synthetic replacements. |
| Synthetic mark | A value written by `ato fix`, or by `pii-eu fix` in 1.x, keeps being recognised as synthetic by every later 2.x release. |

`TestReportsMatchThePublishedSchemas`, `TestFrozenFieldsAreStillWritten` and `TestSARIFKeepsTheLegacyFingerprint`
check these promises on every pull request, `TestSyntheticMarkSurvivesTheRename` pins the synthetic mark, and the
golden files under `internal/report/testdata` pin the exact output of each format.

## Changes in 2.0

2.0.0 renames the product from pii-eu to ATO. Nothing about what is found, how it is scored or how fingerprints
are computed changes. Every 1.x name below that still works is deprecated: it prints a warning on standard error
when it is used and is removed no earlier than 3.0.

| What | 1.x | 2.0 | 1.x name in 2.x |
|---|---|---|---|
| Product name | pii-eu | ATO | – |
| Command and binary | `pii-eu` | `ato` | not installed; scripts must call `ato` |
| Go module | `github.com/Clause-Eight/pii-eu`, command in `cmd/pii-eu` | `github.com/Clause-Eight/ato/v2`, command in `cmd/ato` | – |
| Release archives | `pii-eu_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) | `ato_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows); `checksums.txt` and its signature keep their names | – |
| Settings file | `.pii-eu.toml` | `.ato.toml` | read, with a warning, when `.ato.toml` is absent |
| Baseline file | `.pii-eu-baseline.json` | `.ato-baseline.json` | read, with a warning, when `.ato-baseline.json` is absent; `ato baseline` writes `.ato-baseline.json` and says to delete the old file. The file format and the fingerprints are unchanged. |
| Ignore comment | `pii-eu:ignore <reason>` | `ato:ignore <reason>` | honoured; one warning per scan names how many findings use it and the first file |
| Saved data map | `pii-eu-map.json` in the examples | `ato-map.json` in the examples | `ato` has no default map file: `map --diff FILE` reads any file name, and reads maps with schema version `1.x` and tool name `pii-eu` |
| Error and warning prefix | `pii-eu: ` | `ato: ` | – |
| `version` output | `pii-eu <version>` | `ato <version>` | – |
| JSON report | `schema_version` `1.3`, `tool.name` `pii-eu` | `schema_version` `2.0`, `tool.name` `ato`; no field added, removed or renamed | – |
| Inventory JSON | `schema_version` `1.2`, `tool.name` `pii-eu` | `schema_version` `2.0`, `tool.name` `ato` | – |
| Data map JSON | `schema_version` `1.1`, `tool.name` `pii-eu` | `schema_version` `2.0`, `tool.name` `ato` | – |
| Schema `$id` | `https://github.com/Clause-Eight/pii-eu/schemas/…` | `https://github.com/Clause-Eight/ato/schemas/…` | – |
| SARIF driver | name `pii-eu`, `informationUri` `https://github.com/Clause-Eight/pii-eu` | name `ATO`, `informationUri` `https://github.com/Clause-Eight/ato` | – |
| SARIF partial fingerprint | `piiEu/v1` | `ato/v1` | `piiEu/v1` is still written with the same value, so GitHub code scanning keeps matching alerts opened by 1.x |
| GitLab Code Quality `check_name` | `pii-eu/<detector>` | `ato/<detector>` | – (the `fingerprint` field, which GitLab matches on, is unchanged) |
| GitHub annotation title | `pii-eu: <label>` | `ato: <label>` | – |
| HTML and Markdown reports | "Generated by pii-eu" | "Generated by ATO" | – |
| GitHub Action | `Clause-Eight/pii-eu@<ref>`, name `pii-eu` | `Clause-Eight/ato@<ref>`, name `ATO`; inputs and outputs unchanged | the old repository URL redirects to the renamed repository |
| pre-commit hook | id `pii-eu`, entry `pii-eu scan --staged .` | id `ato`, entry `ato scan --staged .` | id `pii-eu` still exists, runs `ato`, and is named as deprecated |
| GitLab template | `ci/gitlab/pii-eu.gitlab-ci.yml`, job `pii-eu`, `PII_EU_VERSION`, `PII_EU_MIN_CONFIDENCE` | `ci/gitlab/ato.gitlab-ci.yml`, job `ato`, `ATO_VERSION`, `ATO_MIN_CONFIDENCE` | the `PII_EU_` variables are read, with a warning, when the `ATO_` ones are not set; the old file name is gone |
| Bitbucket step | anchor and name `pii-eu`, `PII_EU_VERSION`, report `pii-eu.json` | anchor and name `ato`, `ATO_VERSION`, report `ato.json` | `PII_EU_VERSION` is read, with a warning, when `ATO_VERSION` is not set |
| Test switch for golden files | `PII_EU_UPDATE_GOLDEN=1` | `ATO_UPDATE_GOLDEN=1` | not read (development only) |
| Benchmark script | `compare.py --pii-eu BINARY`, results labelled `pii-eu` | `compare.py --ato BINARY`, results labelled `ato` | the results in `bench/compare/results` keep their `pii-eu` labels |

Unchanged: detector names and rule IDs, flags, settings keys, exit codes, the baseline format and every
fingerprint, the synthetic mark and the values `fix` writes, and every JSON field. The `ato` command reads no
environment variables, in 1.x or 2.x; the `PII_EU_` variables above belong to the CI templates.

## Schema versions

| Release | Findings report | Inventory | Data map |
|---|---|---|---|
| 1.0 | `1.1`: `kind` and `special_category` | `1.0` | `1.0` |
| 1.x | `1.2`: kinds `sink`, `infra` and `lifecycle`; `sink`, `data_category`, `column_name` and the `store` object on findings; `sampled` and `columns` in the summary | `1.1`: the `stores` array; the CSV columns are unchanged | `1.0` |
| 1.x, after #264 | unchanged | `1.2`: source formats `typeorm`, `mongoose`, `doctrine`, `laravel`, `csv`, `tsv` and `jsonl`; stores of type `module` with the optional `module` object; `unknown` as a store's `provider` or `service`, for module calls only | `1.1`: optional `module_source` on `infrastructure` entries |
| 2.0 | `2.0`: `tool.name` is `ato`; no field changes | `2.0`: the same | `2.0`: the same |

Within a major version each schema version only adds optional fields and new enum values, so a reader written for
an earlier `2.x` keeps working: it ignores the new fields, a finding of an unknown `kind` can be read like a
`field` finding, and an unknown `source_format` or store `type` can be read as text. A reader written for `1.x`
reads a `2.0` file once it accepts the new `schema_version` and `tool.name`; no field changed.

## What a minor release may change

- New detectors, new countries, new keywords and better validation. A value that was reported may stop being
  reported when validation gets stricter, and new values may be reported. Pin a version in CI if you need the
  exact same findings.
- New optional JSON fields (the schema version becomes `2.y`), new flags, new settings, new output formats.
- Confidence levels of individual detectors, when the benchmark shows a better setting.
- Turning an off-by-default detector on by default happens only in a minor release and is named in the release
  notes.

## Added in 1.5

- Flags: `--max-rows N` (default 100000; `0` reads every row) and `--image FILE`.
- Settings: `max_rows`, `max_archive_bytes`, `max_archive_entries`, `max_archive_depth` and `image_exclude`.
- JSON report, schema version `1.5`: optional `column_name` on findings, optional `summary.sampled` and
  `summary.columns`. A finding's `path` may hold `!/` to name a file inside an archive or image.
- SARIF results carry `columnName` in their properties when the finding has a column.
- Behaviour, as a minor release may change it: `.gz`, `.tar`, `.tar.gz` and `.tgz` files are scanned inside
  instead of being skipped as binary; CSV, TSV and JSON Lines files and archives are no longer skipped by
  `max_file_size` (data files read their first `max_rows` rows instead); values in files with columns take their
  keywords from their column name instead of the line, so the confidence of such findings may change; fingerprints
  keep their rule (detector, path and line with values replaced).

## Also in 2.0

These changes were planned for the next 1.x release and ship in 2.0.0, next to the rename.

- Exit code `3` also covers files that may hold personal data and were not read in full: data files cut at
  `max_rows`, files over `max_file_size` or an archive limit, archives nested too deep, and document formats
  ATO does not open (office files, PDFs, other compression formats, database files). Before, such a scan could
  exit `0`, which told CI the data was clean when it was never read. This is treated as a correction of the
  meaning of `3`, not a new meaning (ADR 8). `--allow-partial` and `allow_partial = true` restore the old exit
  code for these files.
- `.zip` files are scanned inside, with the same limits as tar files.
- JSON report, schema version `1.3`: optional `not_read` on entries of `summary.files_skipped` and `summary.sampled`.
- Confidence: context (a test path, a line of long numbers, an unlikely word, a time-like name) lowers a valid
  value by one step at most (#256).
- Inventory, schema version `1.2`: TypeORM, Mongoose, Doctrine and Laravel models are read, and Terraform module
  calls that are not read inside are listed under `stores` as type `module` (#264). Data map, schema version
  `1.1`: optional `module_source` on infrastructure entries. Module calls give no scan findings.
- Confidence: order, invoice, request, trace, transaction, shipment and similar words lower a Romanian CNP or
  Slovenian EMŠO without its own keyword. Words that label other numbers now count only on the value's own line,
  not on the line before (#267).
- Cards: a number that is also a valid GTIN-8, -12, -13 or -14 is not reported when gtin, ean, barcode or upc is
  on its line; sku and those words lower other cards one step (#266).
- Values are also found URL-encoded and as whole base64 tokens, and a Romanian CNP is also found in the groups it
  is printed in (1-6-6, 3-4-6, 3-3-3-4, with spaces or dashes). The span of a decoded finding covers the encoded
  text; `ato fix` does not rewrite encoded values (#263).

## Deprecation

- A flag, setting, command, output field or detector to be removed is first marked deprecated in a minor release.
  It keeps working, `ato` prints a warning on standard error when it is used, and the release notes say what
  replaces it.
- It is removed no earlier than the next major release, and at least two minor releases after it was
  deprecated.
- Exit codes and the meaning of existing JSON fields are never changed within a major version.
- The 1.x names that 2.0 replaced (see [Changes in 2.0](#changes-in-20)) are deprecated from 2.0 under the same
  rules: they keep working with a warning and are removed no earlier than 3.0.

## Not covered

- Text output (`--format text`), Markdown and HTML are for people and may change in any release.
- The catalogue of third-party recipients grows and is corrected in minor releases.
- Go packages under `internal/` are not an API.
- The benchmark numbers are measurements, not promises.
