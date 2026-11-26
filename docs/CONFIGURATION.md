# Configuration

Everything ATO can be told: command-line flags, the `.ato.toml` file, exit codes, ignore comments and the optional license key. For what each command does, see [USAGE.md](USAGE.md).

## Command-line flags

| Flag | Meaning |
|---|---|
| `--format` | `text` (default), `json`, `sarif`, `github` (workflow annotations), `codequality` (GitLab), `html` (one self-contained page for people who do not use the command line) |
| `--output FILE` | write the report to a file |
| `--min-confidence` | `low`, `medium` (default) or `high` |
| `--countries` | only these countries' national detectors, as ISO codes or presets: `--countries ro,bg` or `--countries cee`. Cross-border detectors (IBAN, cards, VAT, contact data) always run |
| `--include`, `--exclude` | glob patterns; `**` crosses folders; repeatable |
| `--staged` | scan the staged version of staged files |
| `--baseline FILE` | baseline file (default `.ato-baseline.json` in the scanned folder) |
| `--no-git` | walk the folder instead of asking git (inside a work tree, `.gitignore` is honoured) |
| `--show-values` | print found values on screen; never in JSON, SARIF or files |
| `--max-rows N` | read at most N rows of each CSV, TSV or JSON Lines file (default 100000); `0` reads every row |
| `--image FILE` | scan a saved container image instead of a path |

## Exit codes

Exit codes: `0` nothing found, `1` findings, `2` usage or settings error, `3` the scan is incomplete: a file could not be read, or a file that may hold personal data was not read in full (cut at `max_rows`, over a size or archive limit, or a document format ATO does not open, such as `.xlsx` or `.pdf`). Each such file is listed with the reason. `--allow-partial` (or `allow_partial = true`) keeps the exit code at `0` or `1` for files not read in full; unreadable files still exit `3`.

## The settings file

`.ato.toml` in the scanned folder (a `.pii-eu.toml` from 1.x is still read when there is no `.ato.toml`, with a warning):

```toml
include = ["src/**", "config/**"]
exclude = ["vendor", "**/*.min.js"]
min_confidence = "medium"
max_file_size = 5242880

countries = ["ro", "bg", "hu"]   # or presets: eu, eea, cee, dach, benelux, nordics, baltics

max_rows = 100000             # rows read per data file; 0 reads every row
max_archive_bytes = 4294967296   # uncompressed bytes read from one archive or image
max_archive_entries = 1000000    # entries read from one archive or image
max_archive_depth = 3            # archives inside archives
allow_partial = false            # true: files not read in full do not make the exit code 3
image_exclude = ["/usr/share/", "/usr/lib/"]   # replaces the default; [] scans the whole image

[detectors]
email = false        # switch a detector off
tf-public = false    # Terraform checks are switched off the same way

[min_confidence_per_detector]
eu-phone = "high"    # report phone numbers only when the evidence is strong
```

Presets: `eu` (the 27 member states, the default), `eea` (adds Iceland, Liechtenstein and Norway), `cee`, `dach`, `benelux`, `nordics` and `baltics`. Naming a country outside the EU, such as `ch` or `gb`, switches on its detectors, which are off by default. An unknown code is an error.

## Keeping a deliberate value

Put `ato:ignore` and a reason on the same line or the line before. Without a reason the comment does not apply. The 1.x form `pii-eu:ignore` still works in 2.x and prints a warning; it is deprecated.

```python
SAMPLE_IBAN = "RO49AAAA1B31007593840000"  # ato:ignore documented example from the bank's API guide
```

## Baselines

`ato baseline` records what is found today in `.ato-baseline.json`; later scans report only new findings. Commit the file. Use `--baseline FILE` to keep it elsewhere.

## License key

ATO checks its license on your machine and never contacts a server. Install the license file you received once per machine:

```sh
ato license install acme.license   # checks it and copies it to your config folder
ato license                        # who it is for, the plan and how many days are left
```

In CI, put the license (the line starting `ato-license-v1.`, or the path of the file) in the `ATO_LICENSE` variable instead. A license keeps working for 30 days after its end date, with a notice. Without one ATO still runs, as an evaluation, and says so in one line on stderr; reports and exit codes are the same. How licenses work: [ADR 15](adr/0015-licensing.md).
