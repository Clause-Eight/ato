# Compatibility

From 1.0.0, pii-eu follows semantic versioning. This page says what a 1.x release keeps working, and how a
change is announced before it breaks anything.

## Stable in every 1.x release

| Interface | Promise |
|---|---|
| Commands | `scan`, `baseline`, `inventory`, `map`, `fix`, `detectors` and `version` keep their names and meaning. |
| Flags | Every flag listed by `pii-eu help` keeps its name, its meaning and its default. |
| Exit codes | `0` nothing found, `1` findings, `2` usage or settings error, `3` the scan is incomplete: some files could not be read, or were not read in full (see the next release). `pii-eu fix` exits `0` when it changed files or had nothing to do. |
| Settings file | `.pii-eu.toml` keys keep their names and meaning. An unknown key stays an error. |
| JSON report | `schemas/findings.schema.json`: fields listed there are never removed, renamed or given a new type. `schema_version` stays `1.x`. |
| Inventory | `schemas/inventory.schema.json` and the CSV columns, under the same rule. |
| Data map | `schemas/map.schema.json`, under the same rule; `map --diff` exits `1` when something is new. |
| SARIF | Rule IDs (`<detector>`, `special-category/<category>`, `log-sink/<sink>`, the Terraform checks `tf-unencrypted`, `tf-no-retention`, `tf-non-eea-region`, `tf-public`, and `no-erasure-path`, `no-retention`) and the `piiEu/v1` partial fingerprint. |
| Detector names | A detector's name never changes and is never reused for a different identifier. The same holds for `special-category`, `log-sink`, the `tf-*` checks, `no-erasure-path` and `no-retention`. |
| Fingerprints and baselines | A finding's fingerprint stays the same for the same detector, path and line text, so a committed baseline keeps working. |
| Masking | No output format ever prints a found value, except text output with `--show-values` on screen. `pii-eu fix --dry-run` masks found values and shows only the synthetic replacements. |
| Synthetic mark | A value written by `pii-eu fix` keeps being recognised as synthetic by every later 1.x release. |

`TestReportsMatchThePublishedSchemas` and `TestFrozenFieldsAreStillWritten` check the JSON promises on every
pull request, and the golden files under `internal/report/testdata` pin the exact output of each format.

## Schema versions

| Release | Findings report | Inventory |
|---|---|---|
| 1.0 | `1.1`: `kind` and `special_category` | `1.0` |
| next | `1.2`: kinds `sink`, `infra` and `lifecycle`; `sink`, `data_category`, `column_name` and the `store` object on findings; `sampled` and `columns` in the summary | `1.1`: the `stores` array; the CSV columns are unchanged |

Each version only adds optional fields, so a reader written for an earlier `1.x` keeps working: it ignores the new fields, and a finding of an unknown `kind` can be read like a `field` finding.

## What a minor release may change

- New detectors, new countries, new keywords and better validation. A value that was reported may stop being
  reported when validation gets stricter, and new values may be reported. Pin a version in CI if you need the
  exact same findings.
- New optional JSON fields (the schema version becomes `1.y`), new flags, new settings, new output formats.
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

## Next release

- Exit code `3` also covers files that may hold personal data and were not read in full: data files cut at
  `max_rows`, files over `max_file_size` or an archive limit, archives nested too deep, and document formats
  pii-eu does not open (office files, PDFs, other compression formats, database files). Before, such a scan could
  exit `0`, which told CI the data was clean when it was never read. This is treated as a correction of the
  meaning of `3`, not a new meaning (ADR 8). `--allow-partial` and `allow_partial = true` restore the old exit
  code for these files.
- `.zip` files are scanned inside, with the same limits as tar files.
- JSON report, schema version `1.3`: optional `not_read` on entries of `summary.files_skipped` and `summary.sampled`.
- Confidence: context (a test path, a line of long numbers, an unlikely word, a time-like name) lowers a valid
  value by one step at most (#256).
- Confidence: order, invoice, request, trace, transaction, shipment and similar words lower a Romanian CNP or
  Slovenian EMŠO without its own keyword. Words that label other numbers now count only on the value's own line,
  not on the line before (#267).

## Deprecation

- A flag, setting, command, output field or detector to be removed is first marked deprecated in a minor release.
  It keeps working, `pii-eu` prints a warning on standard error when it is used, and the release notes say what
  replaces it.
- It is removed no earlier than the next major release, and at least two minor releases after it was
  deprecated.
- Exit codes and the meaning of existing JSON fields are never changed within a major version.

## Not covered

- Text output (`--format text`), Markdown and HTML are for people and may change in any release.
- The catalogue of third-party recipients grows and is corrected in minor releases.
- Go packages under `internal/` are not an API.
- The benchmark numbers are measurements, not promises.
