# Usage

What each command does and what ATO reads. Flags and the settings file: [CONFIGURATION.md](CONFIGURATION.md).

## Commands

```bash
ato scan                       # the current folder
ato scan --format json --output ato.json path/to/repo
ato scan --staged .            # staged changes only, for pre-commit
kubectl logs deploy/api | ato scan -     # a log stream
ato scan exports/              # CSV, TSV, JSON Lines and SQL dumps, by column; .gz and .tar inside
ato scan --image app.tar       # a container image saved with `docker save app:latest -o app.tar`
ato baseline                   # accept what is there today; report only new findings
ato inventory                  # every personal-data field in schemas and models, as CSV
ato map                        # a data map for the record of processing
ato fix --dry-run              # show how fixture values would be replaced with synthetic ones
ato fix                        # replace them
```

## Data exports, dumps and archives

ATO reads exports column by column, so a header like `cnp` raises confidence for every value in that column, and a keyword in one column never raises a value in another.

- **CSV and TSV** (`.csv`, `.tsv`, `.tab`): the first line is the header unless it looks like data (a cell with five or more digits); the delimiter (comma, semicolon, tab or pipe) is sniffed from it. Quoted values may hold delimiters and line breaks. Without a header, columns are named `#1`, `#2` and so on.
- **JSON Lines** (`.jsonl`, `.ndjson`): every string and number is named by its key path, such as `customer.cnp`.
- **SQL dumps** (`.sql`, and `.dump`, `.bak`, `.pgsql`, `.psql`, `.mysql` when they are text): `INSERT` values are named by the statement's column list, or by the table's `CREATE TABLE` earlier in the dump; extended inserts and PostgreSQL `COPY ... FROM stdin` blocks work too.

Findings carry the column name (`column_name` in JSON), and the JSON summary groups them by file and column with counts per detector (`summary.columns`). Column headers and JSON keys also go into `ato inventory` and, for special categories such as `diagnosis`, into scan findings.

Large data files are sampled: only the first `max_rows` rows of a CSV, TSV or JSON Lines file are read (default 100000; `--max-rows 0` reads every row), and the file is listed under `summary.sampled`. Data files are not skipped by `max_file_size`; SQL dumps are, so raise `max_file_size` for a large dump.

Gzip, tar and zip files (`.gz`, `.tar`, `.tar.gz`, `.tgz`, `.zip`) are scanned inside, archives in archives too, and findings name the entry: `dump.tar.gz!/data/users.csv`. Archives are not skipped by `max_file_size`; each entry is. `exclude` patterns apply to paths inside archives. Against archive bombs, one archive with everything nested in it may produce at most `max_archive_bytes` uncompressed bytes (4 GiB) and `max_archive_entries` entries (1,000,000), nested at most `max_archive_depth` deep (3). Reading stops at a limit, the archive is listed as unreadable with the reason, and the exit code is 3 when nothing else was found.

## Files that are personal data

Some files hold personal data as a whole or in their metadata. They are reported as `file` findings, which name the file and never a value ([ADR 13](adr/0013-binary-metadata.md)):

- **`database-dump`**: a SQL dump (mysqldump, pg_dump plain or custom format, sqlite3 `.dump`, also inside `.gz` and archives) with rows inserted into columns whose names say they hold personal data. The finding names the table and those columns, such as `customers: email, phone`. High confidence when the file says which tool wrote it, medium for 20 rows or more or a pg_dump archive, low for a few rows of seed data.
- **`document-author`**: a person's name in the author metadata of an office document (`.docx`, `.xlsx`, `.pptx` and their macro and template forms; `.odt`, `.ods`, `.odp`, `.odg`), shown masked. Tool names and generic accounts are left out.
- **`photo-location`**: a JPEG, TIFF, PNG, WebP or HEIF image with an EXIF GPS position, shown masked.
- **`id-document-scan`**: an image or PDF whose file name says it is a passport, an identity card, a driving licence or a residence permit, in the EU languages (`passport`, `carte_identite`, `personalausweis`, `buletin`, `dowod_osobisty`, ...). Medium confidence for a word in the file name, low for an ambiguous word or a folder name.

ATO does no OCR: the content of an image is never read as text, so a scan named `IMG_0001.jpg` is not found. A PDF's own text is searched for machine readable zones (`mrz`). Office documents, PDFs and pg_dump archives are still listed as not read in full, since only their metadata, MRZ text or table of contents was read. Switch any of these off under `[detectors]`, such as `photo-location = false`.

## Container images

`ato scan --image app.tar` reads an image saved with `docker save` (the classic `manifest.json` layout and the OCI layout with `index.json`; gzip-compressed layers included). Decompress an image saved through `gzip` first. Each layer is scanned in parallel, the image configuration (environment, labels, build history) too, and findings name the layer by its short digest: `app.tar!/3f2a9c1b7d4e/etc/app/config.yml`.

A file deleted or replaced in a later layer is still in the image, so it is still reported, and the finding says which layer deleted it. `include` and `exclude` match paths inside the image (`etc/app/**`). Operating system package folders (`/usr/share/`, `/usr/lib/`, `/lib/`, `/var/lib/dpkg/` and the like) are skipped by default; set `image_exclude` to change that list, or to `[]` to scan everything.

## Schemas, special categories and a data inventory

ATO also reads field names from schemas and models: SQL migrations (`CREATE TABLE`, `ALTER TABLE ... ADD`), Laravel migrations (`Schema::create`, `Schema::table`), Django, SQLAlchemy, ActiveRecord, GORM, JPA, Doctrine (attributes and annotations), TypeORM and Prisma models, Mongoose schemas, OpenAPI and JSON Schema documents, protobuf messages and dbt `schema.yml` files. Field names are matched against a dictionary in all 24 EU languages; values are never read for this.

- `ato scan` reports fields whose names say they hold special-category data under GDPR Art. 9 (health, genetic, biometric, racial or ethnic origin, political opinions, religious or philosophical beliefs, trade union membership, sex life or sexual orientation) or data about criminal convictions and offences (Art. 10) as `special-category` findings, with the category. Switch them off with `special-category = false` under `[detectors]`.
- It also finds the data itself: ICD-10, ICD-11, ATC and Italian AIC codes next to a medical word in a record that names a person (`icd10`, `icd11`, `atc-code`, `it-aic`), and `record` findings for HL7 v2 patient segments, FHIR resources about a patient, VCF and genotype files, and biometric templates such as face embeddings. A code on its own is never reported. See [docs/DETECTORS.md](DETECTORS.md) and [ADR 14](adr/0014-art9-records.md).
- `ato inventory` writes every personal-data field it finds as CSV (default), HTML (`--format html`) or JSON (`--format json`, see [schemas/inventory.schema.json](../schemas/inventory.schema.json)): field, table or model, category of personal data, special category, source format, file and line. It is a first draft for a record of processing activities (Art. 30), not a replacement for one.

```console
$ ato inventory --format csv db/
Field,Table or model,Category of personal data,Special category (Art. 9),Source format,Source file,Line
first_name,patients,name,,sql,db/migrations/001_patients.sql,3
diagnosis,patients,special-category,health,sql,db/migrations/001_patients.sql,4
```

## A data map for the record of processing

`ato map` drafts a data map from the code: per component (a folder under `services/`, `apps/`, `packages/` and similar, or a top-level folder), the tables and models holding personal data and their categories, where personal data is written to logs or telemetry, and the third-party services the code names in dependency files, imports and API host names (payments, email, analytics, error tracking, support, AI providers and more; `established_in_eea` says whether the provider is established in the EEA). Every line cites its source file and line. Formats: `markdown` (default), `json` ([schemas/map.schema.json](../schemas/map.schema.json)) and `html`.

For each store the map also shows whether the code can erase a person's data (GDPR Art. 17: a delete, an anonymising update, or a cascade from a parent that can be erased) and whether old data is removed (Art. 5(1)(e): a time-based delete, a TTL index, a partition drop, a DynamoDB time to live, a Redis key expiry or an Elasticsearch or OpenSearch index policy with a delete phase), in SQL, the supported ORMs and the Redis, DynamoDB and search clients. To report stores without either as findings, switch them on:

```toml
[detectors]
no-erasure-path = true
no-retention = true
```

The map is a draft for the record of processing activities (GDPR Art. 30). Purposes, legal bases and retention periods need a person; naming a recipient in code does not prove personal data goes to it.

```bash
ato map --format json --output ato-map.json   # save the map, e.g. on the main branch
ato map --diff ato-map.json                   # in a pull request: exit 1 if a category, store, recipient or log of personal data is new
ato map --diff ato-map.json --format html --output map.html   # the same, as a page for the DPO
```

The HTML page is written for a data protection officer. It opens with what changed since the saved map (new kinds of personal data, new special-category data, new outside companies, new stores and stores that lost encryption, a retention rule, an EU region or an erasure path, and what was removed), then lists what needs a decision, each with why it matters, what to decide and a link to its file and line. Without `--diff`, the decisions come from the whole map. The full map follows, grouped by store, with categories in words. The page is one file with no external resources, prints cleanly and follows the system's light or dark setting.

## Data stores in Terraform

ATO reads the databases, buckets, queues, streams, caches and log groups declared in `.tf` files, for AWS, Google Cloud and Azure, without running Terraform: no binary, no providers, no credentials. For each store it reads encryption at rest, retention (lifecycle rules, retention periods, TTL), backups or versioning, public access and the region, following variables with defaults, `terraform.tfvars`, `*.auto.tfvars`, locals and references between resources in the same folder, including the provider block's region. A store also has a retention limit when the code sets one on it: a DynamoDB time to live switched on through the SDK or the CDK for its table, Redis keys named like a cache written with an expiry, or an index policy with a delete phase for indices named like a search domain.

`ato scan` reports four checks as `infra` findings:

| Check | Reported when |
|---|---|
| `tf-unencrypted` | encryption at rest is switched off, or not set where the service leaves it off |
| `tf-no-retention` | nothing expires the data: no lifecycle rule, retention period or TTL |
| `tf-non-eea-region` | a region is outside the EEA, such as `us-east-1`; one step lower for a country with an EU adequacy decision, such as `eu-west-2` (London) or `eu-central-2` (Zurich) |
| `tf-public` | a public ACL, a grant to `allUsers` or an internet-facing endpoint opens the store |

A finding is high when the store is linked to personal data, and medium otherwise. A store is linked by a tag or label (`data_classification = "pii"`, `pii = true` and similar) or when its name matches a table or model in the inventory: a bucket `acme-patient-exports` is linked to a `patients` table. A setting that comes from something the code does not show, such as a variable without a default, is never treated as missing: it is reported at low confidence, and only for linked stores. Put `ato:ignore` and a reason on the line before a resource to accept it, or switch a check off under `[detectors]`.

```text
$ ato scan infra/
infra/main.tf
  9:1  high    Data store outside the EEA  aws_s3_bucket.customer_exports
          region us-east-1 is outside the EEA (from the aws provider block); linked to personal data: tagged data_classification with "pii"
```

`ato inventory --format json` lists every store under `stores`, with its regions and a yes, no or unknown for each setting. A `module` block whose source is not read (a registry or git module) is listed as type `module` with its source and the setting-like arguments it passes, every setting unknown, when its source, label or arguments point to a data store; a local module that was scanned is read as its own folder. The supported resource types are listed in [docs/DETECTORS.md](DETECTORS.md#terraform-data-stores); how the reader works and where it stops is in [docs/adr/0004-terraform-stores.md](adr/0004-terraform-stores.md).

## Personal data in logs, error trackers and analytics

`ato scan` also reads source code in Go, Python, JavaScript and TypeScript, Java, Kotlin, Ruby and PHP for log, print, error-tracker (Sentry, Bugsnag, Rollbar, Crashlytics and others) and analytics (Segment, Mixpanel, Amplitude, PostHog, Google tag and others) calls. A call is a `log-sink` finding when it is passed:

- a field whose name says it holds personal data, such as `user.email`, `customer.getIban()` or `{"phone": ...}`: medium confidence, with the data category;
- a whole request body, such as `req.body`, `request.json`, `$request->all()` or `$_POST`: medium confidence;
- a whole user-like object, such as `user` or `currentUser`: low confidence.

Each of these also counts when it reaches the call through one local variable in the same function (`v := u.Email` and then `log.Println(v)`, within 30 lines), including inside a string template such as `"#{v}"` or `"${v}"`; the finding then says which variable and on which line it was assigned. A variable that is reassigned, or rebound by a loop or a lambda, is forgotten.

Session replay (Hotjar, FullStory, LogRocket, Microsoft Clarity, Smartlook, PostHog, Sentry Replay, Datadog RUM) is a `log-sink` finding with the sink `session-replay` when scripts, pages or templates switch input masking off (`maskAllInputs: false`, `maskAllText: false`, `defaultPrivacyLevel: 'allow'`, Smartlook recording forms), or let one form field out of it (`data-hj-allow`, `data-clarity-unmask="true"`, `fs-unmask`, `data-dd-privacy="allow"`): medium confidence for a setting or a field named like personal data, low for an unnamed field.

The finding names the sink (`log`, `error-tracker`, `analytics` or `session-replay`) and shows the expression passed or the setting, never a value. Names that are about a value rather than the value (`isEmail`, `emailTemplate`, `len(user.email)`, `hash(user.phone)`) are left alone, and ambiguous words such as `address` count only on a person (`customer.address`, not `server.address`). Switch these findings off with `log-sink = false` under `[detectors]`.

```text
app/signup.py
  42:30  medium  Personal data written to a log  user.email
          the name contains "email"; passed to logger.info, which writes to a log
```

## Fixing test fixtures

`ato fix [path]` replaces the values a scan reports in test, fixture and example paths with synthetic ones from the same generators the test suite uses. It takes the scan flags (`--min-confidence`, `--include`, `--countries` and so on), and:

- each replacement passes the same detector's checks, keeps the original's separators, case, length and letter and digit positions where the format allows, and is the same for the same value in every file and every run;
- each replacement keeps what tests commonly check: an IBAN's country and account layout, a card's brand, a phone number's country code and leading digits, a VAT number's country, the sex and century of a CNP or EGN (with a new, valid birth date), an IP address's version, a wallet's address type, a plate's or postcode's leading letters. An email address keeps the shape of its local part and its top-level domain (with a short second level such as `co.uk`), and gets a made-up domain name rather than `example.com`: addresses on reserved example domains are not reported at all, so they would no longer be recognised as samples;
- where a format has too few values for one to carry the mark (a four-digit postcode, an IPv4 address with short octets), the mark comes first: the replacement may change the format, and a short postcode its country;
- each replacement carries a synthetic mark, so a later scan reports it as a sample at low confidence and `fix` leaves it alone. A real value carries the mark by chance about once in 65,536 values; it is then reported at low confidence instead of being hidden;
- `--dry-run` prints the changes as a unified diff, with found values masked, and changes nothing;
- `--all` also replaces values outside test, fixture and example paths. Review that diff first: it rewrites code and configuration.

Values that are ignored by comment or in the baseline are left alone. For a few detectors with few possible values (dates of birth, names) a marked value cannot always be found; `fix` still replaces the value and says how many replacements are unmarked.

```console
$ ato fix --dry-run
--- a/tests/fixtures/users.json
+++ b/tests/fixtures/users.json
@@ -14 +14 @@
-    "cnp": "•••••••••••••",
+    "cnp": "6081006222714",
Would replace 1 value in 1 file.
```
