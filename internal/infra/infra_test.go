package infra

import (
	"sort"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// All Terraform below is made up for these tests.

func storesOf(t *testing.T, files map[string]string) map[string]domain.Store {
	t.Helper()
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var parsed []*File
	for _, n := range names {
		parsed = append(parsed, Parse(n, []byte(files[n])))
	}
	out := map[string]domain.Store{}
	for _, s := range Stores(parsed) {
		out[s.Location.Path+":"+s.Address()] = s
	}
	return out
}

const euProvider = `provider "aws" {
  region = "eu-central-1"
}
`

type field func(domain.Store) domain.Setting

var (
	encryption field = func(s domain.Store) domain.Setting { return s.Encryption }
	retention  field = func(s domain.Store) domain.Setting { return s.Retention }
	backup     field = func(s domain.Store) domain.Setting { return s.Backup }
	public     field = func(s domain.Store) domain.Setting { return s.Public }
	inEEA      field = func(s domain.Store) domain.Setting { return s.InEEA }
)

func TestSettings(t *testing.T) {
	cases := []struct {
		name  string
		tf    string
		addr  string
		field field
		want  domain.Tri
		why   string
	}{
		{"rds unencrypted by default", `resource "aws_db_instance" "main" {
  engine = "postgres"
}`, "aws_db_instance.main", encryption, domain.No, "not set"},
		{"rds encrypted", `resource "aws_db_instance" "main" {
  storage_encrypted = true
}`, "aws_db_instance.main", encryption, domain.Yes, "= true"},
		{"rds encryption from a variable without default", `variable "encrypt" {}
resource "aws_db_instance" "main" {
  storage_encrypted = var.encrypt
}`, "aws_db_instance.main", encryption, domain.Unknown, "var.encrypt"},
		{"rds encryption from a variable default", `variable "encrypt" {
  default = false
}
resource "aws_db_instance" "main" {
  storage_encrypted = var.encrypt
}`, "aws_db_instance.main", encryption, domain.No, "= false"},
		{"rds encryption from a conditional is never guessed", `resource "aws_db_instance" "main" {
  storage_encrypted = var.env == "prod" ? true : false
}`, "aws_db_instance.main", encryption, domain.Unknown, "cannot be read"},
		{"rds replica keeps the source encryption", `resource "aws_db_instance" "replica" {
  replicate_source_db = "source-db"
}`, "aws_db_instance.replica", encryption, domain.Unknown, "replicate_source_db"},
		{"rds public", `resource "aws_db_instance" "main" {
  publicly_accessible = true
}`, "aws_db_instance.main", public, domain.Yes, ""},
		{"rds private by default", `resource "aws_db_instance" "main" {}`, "aws_db_instance.main", public, domain.No, ""},
		{"aurora serverless v1", `resource "aws_rds_cluster" "c" {
  engine_mode = "serverless"
}`, "aws_rds_cluster.c", encryption, domain.Yes, "always encrypted"},
		{"docdb unencrypted", `resource "aws_docdb_cluster" "d" {}`, "aws_docdb_cluster.d", encryption, domain.No, ""},
		{"redshift default unknown", `resource "aws_redshift_cluster" "w" {}`, "aws_redshift_cluster.w", encryption, domain.Unknown, ""},
		{"dynamodb without ttl", `resource "aws_dynamodb_table" "t" {
  name = "sessions"
}`, "aws_dynamodb_table.t", retention, domain.No, "no ttl block"},
		{"dynamodb with ttl", `resource "aws_dynamodb_table" "t" {
  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }
}`, "aws_dynamodb_table.t", retention, domain.Yes, "ttl.enabled = true"},
		{"dynamodb ttl from a dynamic block", `resource "aws_dynamodb_table" "t" {
  dynamic "ttl" {
    for_each = var.ttl
    content {
      enabled = true
    }
  }
}`, "aws_dynamodb_table.t", retention, domain.Unknown, "dynamic"},
		{"dynamodb always encrypted", `resource "aws_dynamodb_table" "t" {}`, "aws_dynamodb_table.t", encryption, domain.Yes, ""},
		{"dynamodb point in time recovery", `resource "aws_dynamodb_table" "t" {
  point_in_time_recovery { enabled = true }
}`, "aws_dynamodb_table.t", backup, domain.Yes, ""},
		{"log group kept forever", `resource "aws_cloudwatch_log_group" "app" {
  name = "/app/api"
}`, "aws_cloudwatch_log_group.app", retention, domain.No, "kept forever"},
		{"log group zero days", `resource "aws_cloudwatch_log_group" "app" {
  retention_in_days = 0
}`, "aws_cloudwatch_log_group.app", retention, domain.No, "0 keeps logs forever"},
		{"log group with retention from a local", `locals {
  days = 30
}
resource "aws_cloudwatch_log_group" "app" {
  retention_in_days = local.days
}`, "aws_cloudwatch_log_group.app", retention, domain.Yes, "= 30 days"},
		{"sqs encrypted by default", `resource "aws_sqs_queue" "q" {}`, "aws_sqs_queue.q", encryption, domain.Yes, "SSE-SQS"},
		{"sqs encryption off", `resource "aws_sqs_queue" "q" {
  sqs_managed_sse_enabled = false
}`, "aws_sqs_queue.q", encryption, domain.No, ""},
		{"sns without key", `resource "aws_sns_topic" "t" {}`, "aws_sns_topic.t", encryption, domain.No, ""},
		{"sns with key from a resource", `resource "aws_sns_topic" "t" {
  kms_master_key_id = aws_kms_key.k.arn
}`, "aws_sns_topic.t", encryption, domain.Yes, ""},
		{"kinesis default", `resource "aws_kinesis_stream" "s" {}`, "aws_kinesis_stream.s", encryption, domain.No, "NONE"},
		{"kinesis kms", `resource "aws_kinesis_stream" "s" {
  encryption_type = "KMS"
}`, "aws_kinesis_stream.s", encryption, domain.Yes, ""},
		{"elasticache unset", `resource "aws_elasticache_replication_group" "r" {}`, "aws_elasticache_replication_group.r", encryption, domain.Unknown, ""},
		{"opensearch outside a vpc", `resource "aws_opensearch_domain" "o" {
  encrypt_at_rest {
    enabled = false
  }
}`, "aws_opensearch_domain.o", public, domain.Unknown, "vpc_options"},
		{"opensearch unencrypted", `resource "aws_opensearch_domain" "o" {
  encrypt_at_rest {
    enabled = false
  }
}`, "aws_opensearch_domain.o", encryption, domain.No, ""},
		{"ebs follows the account default", `resource "aws_ebs_encryption_by_default" "on" {
  enabled = true
}
resource "aws_ebs_volume" "v" {}`, "aws_ebs_volume.v", encryption, domain.Yes, "aws_ebs_encryption_by_default.on"},
		{"ebs unset", `resource "aws_ebs_volume" "v" {}`, "aws_ebs_volume.v", encryption, domain.Unknown, ""},
		{"efs unencrypted", `resource "aws_efs_file_system" "f" {}`, "aws_efs_file_system.f", encryption, domain.No, ""},

		{"s3 always encrypted", `resource "aws_s3_bucket" "b" {}`, "aws_s3_bucket.b", encryption, domain.Yes, "SSE-S3"},
		{"s3 without lifecycle", `resource "aws_s3_bucket" "b" {}`, "aws_s3_bucket.b", retention, domain.No, "kept forever"},
		{"s3 lifecycle configuration that expires", `resource "aws_s3_bucket" "b" {
  bucket = "example-exports"
}
resource "aws_s3_bucket_lifecycle_configuration" "b" {
  bucket = aws_s3_bucket.b.id
  rule {
    id     = "expire"
    status = "Enabled"
    expiration {
      days = 90
    }
  }
}`, "aws_s3_bucket.b", retention, domain.Yes, "in aws_s3_bucket_lifecycle_configuration.b"},
		{"s3 lifecycle configuration by literal name", `resource "aws_s3_bucket" "b" {
  bucket = "example-exports"
}
resource "aws_s3_bucket_lifecycle_configuration" "b" {
  bucket = "example-exports"
  rule {
    status = "Enabled"
    expiration { days = 30 }
  }
}`, "aws_s3_bucket.b", retention, domain.Yes, ""},
		{"s3 lifecycle that only transitions", `resource "aws_s3_bucket" "b" {}
resource "aws_s3_bucket_lifecycle_configuration" "b" {
  bucket = aws_s3_bucket.b.bucket
  rule {
    status = "Enabled"
    transition {
      days          = 30
      storage_class = "GLACIER"
    }
  }
}`, "aws_s3_bucket.b", retention, domain.No, "none expires"},
		{"s3 disabled lifecycle rule", `resource "aws_s3_bucket" "b" {}
resource "aws_s3_bucket_lifecycle_configuration" "b" {
  bucket = aws_s3_bucket.b.id
  rule {
    status = "Disabled"
    expiration { days = 30 }
  }
}`, "aws_s3_bucket.b", retention, domain.No, ""},
		{"s3 inline lifecycle", `resource "aws_s3_bucket" "b" {
  lifecycle_rule {
    enabled = true
    expiration {
      days = 7
    }
  }
}`, "aws_s3_bucket.b", retention, domain.Yes, ""},
		{"s3 lifecycle switched by a variable", `resource "aws_s3_bucket" "b" {}
resource "aws_s3_bucket_lifecycle_configuration" "b" {
  bucket = aws_s3_bucket.b.id
  rule {
    status = var.expiry_status
    expiration { days = 30 }
  }
}`, "aws_s3_bucket.b", retention, domain.Unknown, ""},
		{"s3 public acl", `resource "aws_s3_bucket" "b" {}
resource "aws_s3_bucket_acl" "b" {
  bucket = aws_s3_bucket.b.id
  acl    = "public-read"
}`, "aws_s3_bucket.b", public, domain.Yes, "public-read"},
		{"s3 public acl ignored by the access block", `resource "aws_s3_bucket" "b" {
  acl = "public-read"
}
resource "aws_s3_bucket_public_access_block" "b" {
  bucket             = aws_s3_bucket.b.id
  ignore_public_acls = true
}`, "aws_s3_bucket.b", public, domain.No, "ignores public ACLs"},
		{"s3 private by default", `resource "aws_s3_bucket" "b" {}`, "aws_s3_bucket.b", public, domain.No, ""},
		{"s3 versioning", `resource "aws_s3_bucket" "b" {}
resource "aws_s3_bucket_versioning" "b" {
  bucket = aws_s3_bucket.b.id
  versioning_configuration {
    status = "Enabled"
  }
}`, "aws_s3_bucket.b", backup, domain.Yes, ""},

		{"cloud sql open to the internet", `resource "google_sql_database_instance" "db" {
  region = "europe-west3"
  settings {
    ip_configuration {
      authorized_networks {
        name  = "all"
        value = "0.0.0.0/0"
      }
    }
  }
}`, "google_sql_database_instance.db", public, domain.Yes, "0.0.0.0/0"},
		{"cloud sql region from the provider", `provider "google" {
  region = "europe-west1"
}
resource "google_sql_database_instance" "db" {}`, "google_sql_database_instance.db", inEEA, domain.Yes, "provider block"},
		{"gcs bucket in the US", `resource "google_storage_bucket" "b" {
  location = "US"
}`, "google_storage_bucket.b", inEEA, domain.No, "US"},
		{"gcs bucket with a delete rule", `resource "google_storage_bucket" "b" {
  location = "EU"
  lifecycle_rule {
    condition {
      age = 30
    }
    action {
      type = "Delete"
    }
  }
}`, "google_storage_bucket.b", retention, domain.Yes, ""},
		{"gcs bucket public to all users", `resource "google_storage_bucket" "b" {
  location = "EU"
}
resource "google_storage_bucket_iam_member" "viewer" {
  bucket = google_storage_bucket.b.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}`, "google_storage_bucket.b", public, domain.Yes, "allUsers"},
		{"bigquery dataset defaults to the US", `resource "google_bigquery_dataset" "d" {
  dataset_id = "analytics"
}`, "google_bigquery_dataset.d", inEEA, domain.No, "defaults to US"},
		{"bigquery table inherits the dataset location", `resource "google_bigquery_dataset" "d" {
  dataset_id = "analytics"
  location   = "EU"
  default_table_expiration_ms = 2592000000
}
resource "google_bigquery_table" "t" {
  dataset_id = google_bigquery_dataset.d.dataset_id
  table_id   = "customers"
}`, "google_bigquery_table.t", inEEA, domain.Yes, "google_bigquery_dataset.d"},
		{"bigquery table inherits the dataset expiry", `resource "google_bigquery_dataset" "d" {
  dataset_id = "analytics"
  location   = "EU"
  default_table_expiration_ms = 2592000000
}
resource "google_bigquery_table" "t" {
  dataset_id = google_bigquery_dataset.d.dataset_id
  table_id   = "customers"
}`, "google_bigquery_table.t", retention, domain.Yes, "in google_bigquery_dataset.d"},
		{"pubsub without storage policy", `resource "google_pubsub_topic" "t" {}`, "google_pubsub_topic.t", inEEA, domain.Unknown, "any region"},
		{"pubsub storage policy", `resource "google_pubsub_topic" "t" {
  message_storage_policy {
    allowed_persistence_regions = ["europe-west1", "us-central1"]
  }
}`, "google_pubsub_topic.t", inEEA, domain.No, "us-central1"},
		{"logging bucket global", `resource "google_logging_project_bucket_config" "l" {
  location = "global"
}`, "google_logging_project_bucket_config.l", inEEA, domain.Unknown, "global"},

		{"azure storage location from the resource group", `resource "azurerm_resource_group" "rg" {
  location = "West Europe"
}
resource "azurerm_storage_account" "sa" {
  location = azurerm_resource_group.rg.location
}`, "azurerm_storage_account.sa", inEEA, domain.Yes, "West Europe"},
		{"azure storage in the US", `resource "azurerm_storage_account" "sa" {
  location = "eastus"
}`, "azurerm_storage_account.sa", inEEA, domain.No, "eastus"},
		{"azure storage public blobs", `resource "azurerm_storage_account" "sa" {
  location                        = "westeurope"
  allow_nested_items_to_be_public = true
}`, "azurerm_storage_account.sa", public, domain.Yes, ""},
		{"azure storage management policy", `resource "azurerm_storage_account" "sa" {
  location = "westeurope"
}
resource "azurerm_storage_management_policy" "p" {
  storage_account_id = azurerm_storage_account.sa.id
  rule {
    name    = "expire"
    enabled = true
    actions {
      base_blob {
        delete_after_days_since_modification_greater_than = 365
      }
    }
  }
}`, "azurerm_storage_account.sa", retention, domain.Yes, ""},
		{"azure sql without tde", `resource "azurerm_mssql_server" "s" {
  location = "northeurope"
}
resource "azurerm_mssql_database" "db" {
  server_id                           = azurerm_mssql_server.s.id
  transparent_data_encryption_enabled = false
}`, "azurerm_mssql_database.db", encryption, domain.No, ""},
		{"azure sql location from the server", `resource "azurerm_mssql_server" "s" {
  location = "uksouth"
}
resource "azurerm_mssql_database" "db" {
  server_id = azurerm_mssql_server.s.id
}`, "azurerm_mssql_database.db", inEEA, domain.No, "uksouth"},
		{"cosmos geo locations", `resource "azurerm_cosmosdb_account" "c" {
  location = "westeurope"
  geo_location {
    location          = "westeurope"
    failover_priority = 0
  }
  geo_location {
    location          = "eastus"
    failover_priority = 1
  }
}`, "azurerm_cosmosdb_account.c", inEEA, domain.No, "eastus"},
		{"log analytics default retention", `resource "azurerm_log_analytics_workspace" "w" {
  location = "germanywestcentral"
}`, "azurerm_log_analytics_workspace.w", retention, domain.Yes, "30 days"},
		{"postgres flexible public", `resource "azurerm_postgresql_flexible_server" "p" {
  location                      = "swedencentral"
  public_network_access_enabled = true
}`, "azurerm_postgresql_flexible_server.p", public, domain.Yes, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stores := storesOf(t, map[string]string{"infra/main.tf": tc.tf})
			s, ok := stores["infra/main.tf:"+tc.addr]
			if !ok {
				t.Fatalf("no store %s in %v", tc.addr, stores)
			}
			got := tc.field(s)
			if got.State != tc.want || !strings.Contains(got.Why, tc.why) {
				t.Fatalf("got %s (%s), want %s with %q", got.State, got.Why, tc.want, tc.why)
			}
		})
	}
}

func TestRegionFromProviderAndVariables(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  domain.Tri
		adeq  string
	}{
		{"provider literal", map[string]string{"m/providers.tf": euProvider, "m/main.tf": `resource "aws_s3_bucket" "b" {}`}, domain.Yes, ""},
		{"provider from a variable default", map[string]string{"m/providers.tf": `provider "aws" {
  region = var.region
}
variable "region" {
  default = "us-east-1"
}`, "m/main.tf": `resource "aws_s3_bucket" "b" {}`}, domain.No, ""},
		{"tfvars override the default", map[string]string{"m/providers.tf": `provider "aws" {
  region = var.region
}
variable "region" {
  default = "us-east-1"
}`, "m/terraform.tfvars": `region = "eu-west-1"`, "m/main.tf": `resource "aws_s3_bucket" "b" {}`}, domain.Yes, ""},
		{"provider from a variable without default", map[string]string{"m/providers.tf": `provider "aws" {
  region = var.region
}
variable "region" {}`, "m/main.tf": `resource "aws_s3_bucket" "b" {}`}, domain.Unknown, ""},
		{"no provider in the folder", map[string]string{"m/main.tf": `resource "aws_s3_bucket" "b" {}`}, domain.Unknown, ""},
		{"provider in another folder does not count", map[string]string{"other/providers.tf": euProvider, "m/main.tf": `resource "aws_s3_bucket" "b" {}`}, domain.Unknown, ""},
		{"aliased provider", map[string]string{"m/providers.tf": euProvider + `provider "aws" {
  alias  = "london"
  region = "eu-west-2"
}`, "m/main.tf": `resource "aws_s3_bucket" "b" {
  provider = aws.london
}`}, domain.No, "GB"},
		{"resource region argument", map[string]string{"m/main.tf": euProvider + `resource "aws_s3_bucket" "b" {
  region = "eu-central-2"
}`}, domain.No, "CH"},
		{"other resources leave the bucket alone", map[string]string{"m/main.tf": euProvider + `resource "aws_s3_bucket" "b" {}
resource "aws_dynamodb_table" "x" {}`}, domain.Yes, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := storesOf(t, tc.files)["m/main.tf:aws_s3_bucket.b"]
			if !ok {
				t.Fatal("bucket not found")
			}
			if s.InEEA.State != tc.want || s.Adequacy != tc.adeq {
				t.Fatalf("got %s %q (%s), want %s %q", s.InEEA.State, s.Adequacy, s.InEEA.Why, tc.want, tc.adeq)
			}
		})
	}
}

func TestDynamoReplicas(t *testing.T) {
	s := storesOf(t, map[string]string{"m/main.tf": euProvider + `resource "aws_dynamodb_table" "t" {
  name = "orders"
  replica {
    region_name = "eu-west-1"
  }
  replica {
    region_name = "ap-southeast-1"
  }
}`})["m/main.tf:aws_dynamodb_table.t"]
	if s.InEEA.State != domain.No || strings.Join(s.Regions, ",") != "eu-central-1,eu-west-1,ap-southeast-1" {
		t.Fatalf("got %s %v (%s)", s.InEEA.State, s.Regions, s.InEEA.Why)
	}
}

func TestNamesTagsAndPersonalData(t *testing.T) {
	stores := storesOf(t, map[string]string{"m/main.tf": `provider "aws" {
  region = "eu-west-1"
  default_tags {
    tags = {
      team = "platform"
    }
  }
}
variable "env" {}
locals {
  common = { owner = "platform" }
}
resource "aws_s3_bucket" "exports" {
  bucket = "${var.env}-patient-exports"
  tags = merge(local.common, {
    data_classification = "PII"
  })
}
resource "aws_dynamodb_table" "carts" {
  name = "carts"
  tags = {
    Name = "carts-table"
    pii  = false
  }
}
resource "google_storage_bucket" "photos" {
  name     = "profile-photos"
  location = "EU"
  labels = {
    contains_pii = "true"
  }
}`})
	b := stores["m/main.tf:aws_s3_bucket.exports"]
	if strings.Join(b.Names, ",") != "patient-exports" {
		t.Errorf("partial name: %v", b.Names)
	}
	if !b.PersonalData || !strings.Contains(b.PersonalWhy, "data_classification") {
		t.Errorf("classification tag: %v %q", b.PersonalData, b.PersonalWhy)
	}
	if b.Tags["team"] != "platform" || b.Tags["owner"] != "platform" {
		t.Errorf("default and merged tags: %v", b.Tags)
	}
	c := stores["m/main.tf:aws_dynamodb_table.carts"]
	if c.PersonalData || strings.Join(c.Names, ",") != "carts,carts-table" {
		t.Errorf("carts: %v %v", c.PersonalData, c.Names)
	}
	if p := stores["m/main.tf:google_storage_bucket.photos"]; !p.PersonalData {
		t.Error("labels mark personal data on Google Cloud")
	}
}

func TestNotAStore(t *testing.T) {
	stores := storesOf(t, map[string]string{"m/main.tf": `resource "aws_iam_role" "r" {}
data "aws_s3_bucket" "existing" {
  bucket = "x"
}
module "network" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
  cidr    = "10.0.0.0/16"
}
module "db" {
  source = "./db"
}`, "m/db/main.tf": `resource "aws_iam_role" "inner" {}`})
	if len(stores) != 0 {
		t.Fatalf("got %v", stores)
	}
}

func TestModuleCalls(t *testing.T) {
	stores := storesOf(t, map[string]string{
		"m/main.tf": euProvider + `
module "patients_db" {
  source            = "terraform-aws-modules/rds/aws"
  version           = "6.0.0"
  identifier        = "patients"
  storage_encrypted = true
  kms_key_id        = var.key
  backup_retention_period = 7
  publicly_accessible     = false
  tags = {
    data_classification = "pii"
  }
}

module "exports" {
  source = "git::https://example.com/acme/terraform-modules.git//storage?ref=v1.2.0"
}

module "archive" {
  source            = "app.example.com/acme/thing/google"
  retention_in_days = 30
}

module "shared" {
  source = "../../shared/modules/audit-bucket"
}

module "local_read" {
  source = "./bucket"
}
`,
		"m/bucket/main.tf": `resource "aws_s3_bucket" "inner" {
  bucket = "inner"
}`,
	})
	db, ok := stores["m/main.tf:module.patients_db"]
	if !ok {
		t.Fatalf("the rds module is a store: %v", stores)
	}
	if db.Type != domain.ModuleType || db.Module != "terraform-aws-modules/rds/aws" || db.Provider != "aws" || db.Service != "database" {
		t.Errorf("module call: %+v", db)
	}
	if strings.Join(db.ModuleSettings, ",") != "backup_retention_period,kms_key_id,publicly_accessible,storage_encrypted" {
		t.Errorf("settings: %v", db.ModuleSettings)
	}
	if strings.Join(db.Names, ",") != "patients" || !db.PersonalData {
		t.Errorf("names and tags: %v %v", db.Names, db.PersonalData)
	}
	for name, s := range map[string]domain.Setting{"encryption": db.Encryption, "retention": db.Retention, "backup": db.Backup, "public": db.Public, "in_eea": db.InEEA} {
		if s.State != domain.Unknown || !strings.Contains(s.Why, "does not read") {
			t.Errorf("%s: %+v", name, s)
		}
	}
	if !strings.Contains(db.Encryption.Why, "storage_encrypted = true") || !strings.Contains(db.Encryption.Why, "kms_key_id") || db.Encryption.Line == 0 {
		t.Errorf("passed settings: %+v", db.Encryption)
	}
	if !strings.Contains(db.Backup.Why, "backup_retention_period = 7") {
		t.Errorf("backup retention is a backup setting: %+v", db.Backup)
	}
	if e := stores["m/main.tf:module.exports"]; e.Service != "object-storage" || e.Provider != "aws" {
		t.Errorf("git source with a subfolder, provider from the folder: %+v", e)
	}
	if a := stores["m/main.tf:module.archive"]; a.Service != "unknown" || a.Provider != "google" || strings.Join(a.ModuleSettings, ",") != "retention_in_days" {
		t.Errorf("settings without a known service: %+v", a)
	}
	if s := stores["m/main.tf:module.shared"]; s.Module != "../../shared/modules/audit-bucket" || s.Service != "object-storage" {
		t.Errorf("a local module outside the scan is not read: %+v", s)
	}
	if _, ok := stores["m/main.tf:module.local_read"]; ok {
		t.Error("a local module that was scanned is read as its own folder")
	}
	if _, ok := stores["m/bucket/main.tf:aws_s3_bucket.inner"]; !ok {
		t.Error("the local module's own stores are read")
	}
}

func TestCandidate(t *testing.T) {
	yes := []string{"main.tf", "infra/db.tf", "terraform.tfvars", "env/prod.auto.tfvars"}
	no := []string{"main.tf.json", "prod.tfvars", ".terraform/modules/x/main.tf", "infra/.terraform/modules/main.tf", "main.go"}
	for _, p := range yes {
		if !Candidate(p) {
			t.Errorf("%s should be read", p)
		}
	}
	for _, p := range no {
		if Candidate(p) {
			t.Errorf("%s should not be read", p)
		}
	}
}

func TestTypesCoverEachProvider(t *testing.T) {
	types := Types()
	for _, p := range []string{"aws", "google", "azurerm"} {
		if len(types[p]) < 5 {
			t.Errorf("%s: %v", p, types[p])
		}
	}
}
