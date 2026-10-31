package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/walk"
)

// The Terraform and SQL below are made up for these tests.

func scanTree(t *testing.T, files map[string]string, req Request) Result {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	req.Root, req.Walk, req.Workers = dir, walk.Options{NoGit: true}, 2
	res, err := Scan(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

const storesTF = `provider "aws" {
  region = "us-east-1"
}

variable "kms" {}

resource "aws_db_instance" "patients" {
  identifier = "patients-db"
}

resource "aws_db_instance" "metrics" {
  identifier = "metrics-db"
}

resource "aws_sns_topic" "alerts" {
  name              = "alerts"
  kms_master_key_id = var.kms
}

resource "aws_ebs_volume" "scratch" {
  tags = {
    data_classification = "personal"
  }
}

# ato:ignore metrics are aggregated and hold no personal data
resource "aws_cloudwatch_log_group" "metrics" {
  name = "metrics"
}
`

const patientsSQL = "CREATE TABLE patients (\n  id bigint,\n  full_name text\n);\n"

func infraFindings(res Result) map[string]domain.Finding {
	out := map[string]domain.Finding{}
	for _, f := range res.Findings {
		if f.Kind == domain.KindInfra {
			out[f.Detector+" "+f.Masked] = f
		}
	}
	return out
}

func TestStoreFindingsAndConfidence(t *testing.T) {
	res := scanTree(t, map[string]string{"infra/main.tf": storesTF, "db/001_patients.sql": patientsSQL}, Request{Infra: InfraDetectors()})
	got := infraFindings(res)
	cases := []struct {
		key  string
		want domain.Confidence
		why  string
	}{
		{"tf-unencrypted aws_db_instance.patients", domain.High, `matches "patients"`},
		{"tf-non-eea-region aws_db_instance.patients", domain.High, "us-east-1"},
		{"tf-unencrypted aws_db_instance.metrics", domain.Medium, "not linked"},
		{"tf-unencrypted aws_ebs_volume.scratch", domain.Low, "check this by hand"},
	}
	for _, tc := range cases {
		f, ok := got[tc.key]
		if !ok {
			t.Errorf("missing %s in %v", tc.key, keysOf(got))
			continue
		}
		if f.Confidence != tc.want || !strings.Contains(strings.Join(f.Reasons, "; "), tc.why) {
			t.Errorf("%s: %s %v", tc.key, f.Confidence, f.Reasons)
		}
		if f.Store == nil || f.Location.Path != "infra/main.tf" || f.Fingerprint == "" || f.Country != domain.CrossBorder {
			t.Errorf("%s: %+v", tc.key, f)
		}
	}
	if f, ok := got["tf-unencrypted aws_sns_topic.alerts"]; ok {
		t.Errorf("a key set from a variable is not missing encryption: %+v", f)
	}
	if f := got["tf-no-retention aws_cloudwatch_log_group.metrics"]; !f.Ignored {
		t.Errorf("the ignore comment above the resource applies: %+v", f)
	}
	if f := got["tf-unencrypted aws_db_instance.patients"]; f.Location.Line != 7 {
		t.Errorf("an unset attribute points at the resource: line %d", f.Location.Line)
	}
	if len(res.Stores) != 0 || len(res.Inventory) != 0 {
		t.Error("stores and fields are returned only for an inventory")
	}
}

func TestStoreChecksCanBeSwitchedOff(t *testing.T) {
	res := scanTree(t, map[string]string{"infra/main.tf": storesTF}, Request{Infra: []string{"tf-no-retention"}})
	for _, f := range res.Findings {
		if f.Detector != "tf-no-retention" {
			t.Errorf("switched off: %s", f.Detector)
		}
	}
	res = scanTree(t, map[string]string{"infra/main.tf": storesTF}, Request{})
	if len(res.Findings) != 0 {
		t.Errorf("no checks enabled: %+v", res.Findings)
	}
}

func TestInventoryHoldsStores(t *testing.T) {
	res := scanTree(t, map[string]string{"infra/main.tf": storesTF, "db/001_patients.sql": patientsSQL}, Request{Inventory: true})
	if len(res.Stores) != 5 {
		t.Fatalf("stores: %+v", res.Stores)
	}
	linked := map[string]bool{}
	for _, s := range res.Stores {
		linked[s.Address()] = s.PersonalData
	}
	if !linked["aws_db_instance.patients"] || !linked["aws_ebs_volume.scratch"] || linked["aws_db_instance.metrics"] {
		t.Errorf("links: %v", linked)
	}
	for _, f := range res.Findings {
		if f.Kind == domain.KindInfra {
			t.Errorf("an inventory without checks reports no store findings: %+v", f)
		}
	}
}

func TestAdequacyAndTestPathsLowerConfidence(t *testing.T) {
	tf := `provider "aws" {
  region = "eu-west-2"
}
resource "aws_dynamodb_table" "users" {
  name = "users"
  ttl {
    attribute_name = "expires"
    enabled        = true
  }
  tags = {
    pii = "true"
  }
}
`
	res := scanTree(t, map[string]string{"infra/main.tf": tf}, Request{Infra: InfraDetectors()})
	got := infraFindings(res)
	if len(got) != 1 {
		t.Fatalf("got %v", keysOf(got))
	}
	f := got["tf-non-eea-region aws_dynamodb_table.users"]
	if f.Confidence != domain.Medium || !strings.Contains(strings.Join(f.Reasons, ";"), "GB has an EU adequacy decision") {
		t.Errorf("adequacy: %s %v", f.Confidence, f.Reasons)
	}
	res = scanTree(t, map[string]string{"examples/main.tf": tf}, Request{Infra: InfraDetectors()})
	if f = infraFindings(res)["tf-non-eea-region aws_dynamodb_table.users"]; f.Confidence != domain.Low || !f.TestPath {
		t.Errorf("example path: %s", f.Confidence)
	}
}

func TestStoreNamesMatchInventory(t *testing.T) {
	cases := []struct {
		container string
		names     []string
		want      bool
	}{
		{"patients", []string{"eu-patients-exports"}, true},
		{"Patient", []string{"patients"}, true},
		{"PatientRecord", []string{"acme-patient-records"}, true},
		{"category", []string{"categories"}, true},
		{"users", []string{"user"}, true},
		{"user", []string{"username-cache"}, false},
		{"app", []string{"app"}, false},
		{"orders", []string{"metrics"}, false},
	}
	for _, tc := range cases {
		stores := []domain.Store{{Type: "aws_s3_bucket", Name: "b", Names: tc.names}}
		entries := []domain.InventoryEntry{{Field: domain.Field{Name: "email", Container: tc.container, Location: domain.Location{Path: "db.sql"}}}}
		linkStores(stores, entries)
		if stores[0].PersonalData != tc.want {
			t.Errorf("%s vs %v: got %v", tc.container, tc.names, stores[0].PersonalData)
		}
	}
}

func TestCodeExpiryGivesStoresRetention(t *testing.T) {
	files := map[string]string{
		"infra/main.tf": `provider "aws" {
  region = "eu-central-1"
}
resource "aws_dynamodb_table" "sessions" {
  name = "app-sessions"
}
resource "aws_dynamodb_table" "orders" {
  name = "orders"
}
resource "aws_elasticache_replication_group" "carts" {
  replication_group_id = "carts"
}
resource "aws_db_instance" "visits" {
  identifier = "visits"
}`,
		"app/ttl.py":   "client.update_time_to_live(\n    TableName=\"app-sessions\",\n    TimeToLiveSpecification={\"Enabled\": True, \"AttributeName\": \"ttl\"},\n)\n",
		"app/cache.py": "import redis\nr = redis.Redis()\nr.setex(\"cart:\" + cid, 3600, body)\nr.setex(\"visit:\" + vid, 3600, body)\n",
	}
	for _, req := range []Request{{Infra: InfraDetectors()}, {Infra: InfraDetectors(), Inventory: true, Lifecycle: true}} {
		res := scanTree(t, files, req)
		got := infraFindings(res)
		if f, ok := got["tf-no-retention aws_dynamodb_table.sessions"]; ok {
			t.Errorf("the SDK sets a time to live on the table: %+v", f)
		}
		if _, ok := got["tf-no-retention aws_dynamodb_table.orders"]; !ok {
			t.Errorf("orders has no time to live: %v", keysOf(got))
		}
		if !req.Inventory {
			continue
		}
		for _, s := range res.Stores {
			switch s.Address() {
			case "aws_dynamodb_table.sessions":
				if s.Retention.State != domain.Yes || !strings.Contains(s.Retention.Why, "app/ttl.py:2") {
					t.Errorf("sessions: %+v", s.Retention)
				}
			case "aws_elasticache_replication_group.carts":
				if s.Retention.State != domain.Yes || !strings.Contains(s.Retention.Why, "Redis key expiry") {
					t.Errorf("carts: %+v", s.Retention)
				}
			case "aws_db_instance.visits":
				if s.Retention.State != domain.NotApplicable {
					t.Errorf("a Redis key does not give a database a retention limit: %+v", s.Retention)
				}
			}
		}
	}
}

func keysOf(m map[string]domain.Finding) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
