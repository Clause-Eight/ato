# Compatibility

From 1.0.0, pii-eu follows semantic versioning. This page says what a 1.x release keeps working, and how a
change is announced before it breaks anything.

## Stable in every 1.x release

| Interface | Promise |
|---|---|
| Commands | `scan`, `baseline`, `inventory`, `detectors` and `version` keep their names and meaning. |
| Flags | Every flag listed by `pii-eu help` keeps its name, its meaning and its default. |
| Exit codes | `0` nothing found, `1` findings, `2` usage or settings error, `3` some files could not be read. |
| Settings file | `.pii-eu.toml` keys keep their names and meaning. An unknown key stays an error. |
| JSON report | `schemas/findings.schema.json`: fields listed there are never removed, renamed or given a new type. `schema_version` stays `1.x`. |
| Inventory | `schemas/inventory.schema.json` and the CSV columns, under the same rule. |
| SARIF | Rule IDs (`<detector>`, `special-category/<category>`) and the `piiEu/v1` partial fingerprint. |
| Detector names | A detector's name never changes and is never reused for a different identifier. |
| Fingerprints and baselines | A finding's fingerprint stays the same for the same detector, path and line text, so a committed baseline keeps working. |
| Masking | No output format ever prints a found value, except text output with `--show-values` on screen. |

`TestReportsMatchThePublishedSchemas` and `TestFrozenFieldsAreStillWritten` check the JSON promises on every
pull request, and the golden files under `internal/report/testdata` pin the exact output of each format.

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

## Deprecation

- A flag, setting, command, output field or detector to be removed is first marked deprecated in a minor release.
  It keeps working, `pii-eu` prints a warning on standard error when it is used, and the release notes say what
  replaces it.
- It is removed no earlier than the next major release, and at least two minor releases after it was
  deprecated.
- Exit codes and the meaning of existing JSON fields are never changed within a major version.

## Not covered

- Text output (`--format text`) is for people and may change in any release.
- Go packages under `internal/` are not an API.
- The benchmark numbers are measurements, not promises.
