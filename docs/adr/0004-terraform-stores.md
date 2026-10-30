# 4. Reading data stores from Terraform

Date: 2026-10-27 · Status: accepted

## Context

Privacy reviews ask where personal data is kept, not only which fields hold it: is the database encrypted, does
the bucket ever delete anything, is the region inside the EEA. Most teams declare those stores in Terraform.
pii-eu must answer from the code alone, inside a pre-commit hook or a CI job, with no cloud credentials, no
Terraform binary and no provider downloads, and without growing the single binary much (ADR 1).

## Decision

- **A small HCL reader of our own** in `internal/infra`, standard library only. It reads the native syntax
  (blocks, attributes, strings with interpolation, heredocs, objects, tuples, function calls, comments) and keeps
  any expression it does not understand as "other". The official HCL module would pull in `go-cty`, the
  `zclconf` text libraries and their dependencies for features we do not use (type checking, evaluation of the
  full function library). A reader that skips what it does not understand also degrades better on broken or
  half-written files; a fuzz test checks that no input makes it fail or hang.
- **Resolution stops at the folder.** Terraform treats a folder as one module, so the reader follows
  `var.x` (defaults, `terraform.tfvars`, `*.auto.tfvars`), `local.x`, `merge()` and references between resources
  in the same folder, such as `azurerm_resource_group.main.location` or `aws_s3_bucket.logs.id`. Values from
  `-var-file`, data sources, module outputs, workspaces, Terragrunt and the environment are not read.
- **Three answers, never a guess.** Every setting is yes, no or unknown (or does not apply). Unknown means the
  value comes from something the code does not show, or the service default changed between versions or depends
  on an account setting (EBS default encryption, Redshift and ElastiCache defaults). An unknown is never a
  failure: it becomes a low-confidence finding, and only for stores linked to personal data. A documented,
  stable default counts as written: an RDS instance without `storage_encrypted` is unencrypted, a CloudWatch log
  group without `retention_in_days` keeps logs forever, a BigQuery dataset without `location` is in the US.
- **Service guarantees count.** Stores the provider always encrypts (S3 since January 2023, DynamoDB, Google
  Cloud, Azure Storage) are encrypted. Queues and streams whose service deletes messages after a bounded period
  have a retention limit. Databases have no data retention setting in Terraform, so retention does not apply to
  them; backup retention is reported separately and is not a check.
- **Side resources belong to their store.** `aws_s3_bucket_lifecycle_configuration`, `_versioning`, `_acl` and
  `_public_access_block`, Google bucket and dataset IAM members and Azure storage management policies are joined
  to the store by reference or by literal name.
- **Residency.** A region is in the EEA, outside it in a country with an EU adequacy decision (Switzerland, the
  United Kingdom, Canada, Japan, South Korea, Israel, New Zealand), or outside it without one. London and Zurich
  are outside the EEA: they are findings, one confidence step lower, with the adequacy noted, because the
  transfer is lawful but must still be recorded. The United States counts as a third country because the EU-US
  Data Privacy Framework covers only certified organisations. A region pii-eu does not know is unknown.
- **Linking to personal data.** A store is linked when a tag or label says so (`data_classification`,
  `classification` or `sensitivity` holding pii, personal, gdpr, phi, health or special; `pii`, `personal_data`,
  `contains_pii` or `gdpr` set to true), or when its name or resource label matches a table or model of the
  inventory, word by word and allowing a plural. Linking needs the inventory of the whole scan, so the engine
  links and judges stores after all files are read. Linked stores give high findings, others medium.
- **One more finding kind.** Store findings have kind `infra`, the resource address as `masked`, and a `store`
  object in JSON (schema 1.3). The inventory JSON gains an optional `stores` array (schema 1.1); the CSV keeps
  its columns. Each check is a detector name that can be switched off under `[detectors]`.

## Consequences

- No new dependency. The reader and the resource rules are about two thousand lines, most of them the
  per-provider rules, which are plain tables and small functions.
- Stores created inside registry modules (`source = "terraform-aws-modules/..."`) are not seen; local modules
  are read as their own folders, where values passed in by the caller are unknown. Since #264 a module call that
  is not read and points to a store (by its source, label or setting-like arguments) is listed in the inventory
  as type `module` with every setting unknown, and gives no findings.
- Values that depend on `count`, `for_each`, conditionals or `-var-file` stay unknown, which keeps findings
  honest but quieter than a plan-based tool.
- `.tf.json` files are not read.
- Supporting a new resource type is one entry in the provider's file in `internal/infra` and a test case.
