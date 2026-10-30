package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/datamap"
	"github.com/Clause-Eight/pii-eu/internal/domain"
)

func sampleMap() DataMap {
	return DataMap{Tool: "pii-eu", Version: "test", Map: datamap.Build(datamap.Input{
		Inventory: []domain.InventoryEntry{
			{Field: domain.Field{Name: "first_name", Container: "patients", Location: domain.Location{Path: "services/clinic/db/001.sql", Line: 3}}, Classification: domain.Classification{Category: domain.CategoryName}},
			{Field: domain.Field{Name: "diagnosis", Container: "patients", Location: domain.Location{Path: "services/clinic/db/001.sql", Line: 4}}, Classification: domain.Classification{Category: domain.CategorySpecial, Special: domain.SpecialHealth}},
		},
		Findings: []domain.Finding{{Kind: domain.KindSink, Label: "Personal data written to a log", Category: domain.CategoryContact, Location: domain.Location{Path: "services/billing/api.py", Line: 9}}},
		Recipients: []domain.RecipientUse{
			{Name: "Stripe", Purpose: "payments", Evidence: "dependency", Location: domain.Location{Path: "services/billing/requirements.txt", Line: 2}},
			{Name: "Mistral AI", Purpose: "AI model provider", EEA: true, Evidence: "import", Location: domain.Location{Path: "services/clinic/notes.py", Line: 1}},
		},
		Infra:     []domain.Store{{Type: "aws_s3_bucket", Name: "patient_exports", Provider: "aws", Service: "object-storage", Regions: []string{"us-east-1"}, InEEA: domain.Setting{State: domain.No}, Encryption: domain.Setting{State: domain.Yes}, Retention: domain.Setting{State: domain.No}, Public: domain.Setting{State: domain.No}, PersonalData: true, Location: domain.Location{Path: "services/clinic/infra/main.tf", Line: 4}}},
		Lifecycle: []domain.StoreLifecycle{{Store: "patients", Erasure: []domain.Removal{{How: "SQL DELETE", Location: domain.Location{Path: "services/clinic/repo.sql", Line: 7}}}, Retention: []domain.Removal{}}},
	})}
}

func TestMapMatchesSchemaAndReadsBack(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMap("json", &buf, sampleMap()); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	schema := loadJSON(t, filepath.Join("..", "..", "schemas", "map.schema.json"))
	validate(t, schema, schema, doc, "map")
	back, err := ReadMap(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if d := datamap.Diff(sampleMap().Map, back); len(d) != 0 {
		t.Errorf("read-back map differs: %+v", d)
	}
	if !reflect.DeepEqual(back.Categories(), sampleMap().Map.Categories()) {
		t.Errorf("categories differ after reading back")
	}
}

func TestMapModuleCall(t *testing.T) {
	m := DataMap{Tool: "pii-eu", Version: "test", Map: datamap.Build(datamap.Input{Infra: []domain.Store{syntheticModule()}})}
	var buf bytes.Buffer
	if err := WriteMap("json", &buf, m); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	schema := loadJSON(t, filepath.Join("..", "..", "schemas", "map.schema.json"))
	validate(t, schema, schema, doc, "map")
	back, err := ReadMap(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := back.Components[0].Infra[0]
	if got.Address != "module.orders_db" || got.Module != syntheticModule().Module || got.Encrypted != "unknown" {
		t.Errorf("module call read back as %+v", got)
	}
}

func TestMapGolden(t *testing.T) {
	for _, format := range MapFormats {
		var buf bytes.Buffer
		if err := WriteMap(format, &buf, sampleMap()); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "golden", "map."+format)
		if os.Getenv("PII_EU_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run with PII_EU_UPDATE_GOLDEN=1 to create it)", err)
		}
		if got := strings.ReplaceAll(buf.String(), "\r\n", "\n"); got != strings.ReplaceAll(string(want), "\r\n", "\n") {
			t.Errorf("%s differs from %s:\n%s", format, path, got)
		}
	}
}

// olderMap is sampleMap as it was before: no health data, no AI provider, the bucket in an EU region, encrypted
// and with a retention rule, and a leads table that has since been removed.
func olderMap() *datamap.Map {
	m := datamap.Build(datamap.Input{
		Inventory: []domain.InventoryEntry{
			{Field: domain.Field{Name: "first_name", Container: "patients", Location: domain.Location{Path: "services/clinic/db/001.sql", Line: 3}}, Classification: domain.Classification{Category: domain.CategoryName}},
			{Field: domain.Field{Name: "email", Container: "leads", Location: domain.Location{Path: "services/clinic/db/002.sql", Line: 2}}, Classification: domain.Classification{Category: domain.CategoryContact}},
		},
		Recipients: []domain.RecipientUse{{Name: "Stripe", Purpose: "payments", Evidence: "dependency", Location: domain.Location{Path: "services/billing/requirements.txt", Line: 2}}},
		Infra:      []domain.Store{{Type: "aws_s3_bucket", Name: "patient_exports", Provider: "aws", Service: "object-storage", Regions: []string{"eu-central-1"}, InEEA: domain.Setting{State: domain.Yes}, Encryption: domain.Setting{State: domain.Yes}, Retention: domain.Setting{State: domain.Yes}, Public: domain.Setting{State: domain.No}, PersonalData: true, Location: domain.Location{Path: "services/clinic/infra/main.tf", Line: 4}}},
		Lifecycle:  []domain.StoreLifecycle{{Store: "patients", Erasure: []domain.Removal{{How: "SQL DELETE", Location: domain.Location{Path: "services/clinic/repo.sql", Line: 7}}}, Retention: []domain.Removal{}}},
	})
	return &m
}

func TestMapHTMLWithDiffGolden(t *testing.T) {
	m := sampleMap()
	m.Previous, m.PreviousName = olderMap(), "pii-eu-map.json"
	var buf bytes.Buffer
	if err := WriteMap("html", &buf, m); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden", "map_diff.html")
	if os.Getenv("PII_EU_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with PII_EU_UPDATE_GOLDEN=1 to create it)", err)
	}
	if got := strings.ReplaceAll(buf.String(), "\r\n", "\n"); got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Errorf("html differs from %s:\n%s", path, got)
	}
	for _, s := range []string{
		"What changed since the last saved map", "Health data in services/clinic.", "Mistral AI", "no longer has a rule that deletes old data",
		"is no longer limited to EU regions", "Store leads in services/clinic is gone.", "Needs a decision",
		"Decide the purpose, which Art. 9(2) exception applies", "processor agreement (GDPR Art. 28)",
		`href="./services/clinic/db/001.sql"`, "supporting evidence, not legal advice",
	} {
		if !strings.Contains(buf.String(), s) {
			t.Errorf("html lacks %q", s)
		}
	}
}

func TestMapHTMLIsSelfContainedAndHasNoValues(t *testing.T) {
	// Synthetic values, as a scan would see them; a map must never carry them into the page.
	values := []string{"RO49AAAA1B31007593840000", "1800101221144", "ana.pop@example.invalid"}
	in := datamap.Input{
		Inventory:  []domain.InventoryEntry{{Field: domain.Field{Name: "iban", Container: "accounts", Location: domain.Location{Path: "api/db/001.sql", Line: 2}}, Classification: domain.Classification{Category: domain.CategoryFinancial}}},
		Recipients: []domain.RecipientUse{{Name: "Stripe", Purpose: "payments", Location: domain.Location{Path: "api/requirements.txt", Line: 1}}},
	}
	for i, v := range values {
		in.Findings = append(in.Findings,
			domain.Finding{Kind: domain.KindValue, Value: v, Masked: "****", Location: domain.Location{Path: "api/fixtures.json", Line: i + 1}},
			domain.Finding{Kind: domain.KindSink, Value: v, Masked: "user.email", Label: "Personal data written to a log", Category: domain.CategoryContact, Location: domain.Location{Path: "api/log.py", Line: i + 1}})
	}
	current := datamap.Build(in)
	for _, previous := range []*datamap.Map{nil, olderMap(), {}} {
		var buf bytes.Buffer
		if err := WriteMap("html", &buf, DataMap{Tool: "pii-eu", Version: "test", Map: current, Previous: previous}); err != nil {
			t.Fatal(err)
		}
		page := strings.ToLower(buf.String())
		for _, bad := range []string{"http:", "https:", "src=", "url(", "@import", "<script", "<img", "<link", `href="//`, "file:"} {
			if strings.Contains(page, bad) {
				t.Errorf("page contains %q", bad)
			}
		}
		for _, v := range values {
			if strings.Contains(page, strings.ToLower(v)) {
				t.Errorf("page contains a found value")
			}
		}
	}
}

func TestMapHTMLLinkIsRelative(t *testing.T) {
	for p, want := range map[string]string{
		"services/a b/x.sql":     "./services/a%20b/x.sql",
		"dump.tar.gz!/users.csv": "./dump.tar.gz",
		"javascript:alert(1).js": "./javascript:alert%281%29.js",
		"/abs/path.sql":          "",
		"./db/001.sql":           "./db/001.sql",
		"weird#name?.sql":        "./weird%23name%3F.sql",
	} {
		if got := (place{Path: p}).Href(); got != want {
			t.Errorf("%s: got %q, want %q", p, got, want)
		}
	}
}

func TestReadMapRejectsOtherDocuments(t *testing.T) {
	for _, doc := range []string{`not json`, `{"schema_version": "2.0", "components": []}`} {
		if _, err := ReadMap(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: want an error", doc)
		}
	}
}
