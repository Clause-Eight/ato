package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// sentinel is a synthetic value used only to prove that reports never print raw values.
const sentinel = "SENTINEL-4929000000006"

func sample() Report {
	return Report{
		Tool: "ato", Version: "test", Root: ".",
		Findings: []domain.Finding{
			{Detector: "card", Label: "Payment card number", Country: "EU", Location: domain.Location{Path: "app/config.yml", Line: 3, Column: 9},
				Value: sentinel, Masked: "••••••••••••0006", Confidence: domain.High, Reasons: []string{"brand prefix, length and Luhn check are valid", `the word "card" is nearby`}, Fingerprint: "0123456789abcdef0123456789abcdef"},
			{Detector: "ro-cnp", Label: "Romanian personal number (CNP)", Country: "RO", Location: domain.Location{Path: "tests/fixtures/users.json", Line: 14, Column: 12},
				Value: sentinel, Masked: "•••••••••••••", Confidence: domain.Low, Reasons: []string{"date, county code and check digit are valid", "in a test, fixture or example path"}, TestPath: true, Fingerprint: "fedcba9876543210fedcba9876543210"},
			{Detector: "ro-cnp", Label: "Romanian personal number (CNP)", Country: "RO", Location: domain.Location{Path: "exports/users.csv.gz!/users.csv", Line: 2, Column: 7},
				ColumnName: "cnp", Value: sentinel, Masked: "•••••••••••••", Confidence: domain.Medium, Reasons: []string{"date, county code and check digit are valid", `the word "cnp" is nearby`}, Fingerprint: "00ff00ff00ff00ff00ff00ff00ff00ff"},
			{Detector: "ee-isikukood", Label: "Estonian personal code (isikukood)", Country: "EE", Location: domain.Location{Path: "db/seed.sql", Line: 8, Column: 31},
				Value: sentinel, Masked: "•••••••••••", Confidence: domain.Medium, Reasons: []string{"birth date, century digit and check digit are valid", "the whole of a quoted value", "just as valid as Lithuanian personal code (asmens kodas); nothing nearby says which"},
				Fingerprint: "0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f", Possible: []domain.Possible{{Detector: "lt-asmens-kodas", Label: "Lithuanian personal code (asmens kodas)", Country: "LT"}}},
		},
		Summary: domain.Summary{FilesScanned: 42, FilesSkipped: []domain.Skipped{{Path: "logo.png", Reason: "binary"}},
			Sampled: []domain.Skipped{{Path: "exports/users.csv.gz!/users.csv", Reason: "only the first 100000 rows were read (max_rows)"}}},
	}
}

func TestNoFormatLeaksValues(t *testing.T) {
	for _, format := range Formats() {
		for _, show := range []bool{false, true} {
			var buf bytes.Buffer
			if err := Write(format, &buf, sample(), Options{ShowValues: show}); err != nil {
				t.Fatal(err)
			}
			leaked := strings.Contains(buf.String(), sentinel)
			if leaked && !(format == "text" && show) {
				t.Errorf("%s (show=%v) printed a raw value", format, show)
			}
			if format == "text" && show && !leaked {
				t.Error("--show-values should print values in text output")
			}
		}
	}
}

func TestGolden(t *testing.T) {
	golden(t, "report", withSink(sample()))
	golden(t, "infra", withStore(Report{Tool: "ato", Version: "test", Summary: domain.Summary{FilesScanned: 3}}))
	golden(t, "file", fileReport())
}

// fileReport holds file-level findings: a database dump and a document author, with the document not read.
func fileReport() Report {
	return Report{
		Tool: "ato", Version: "test", Root: ".",
		Findings: []domain.Finding{
			{Kind: domain.KindFile, Category: domain.CategoryContact, Detector: "database-dump", Label: "Database dump with rows of personal data", Country: "EU",
				Location: domain.Location{Path: "backups/shop.sql.gz!/shop.sql", Line: 41, Column: 1}, Masked: "customers: email, phone", Confidence: domain.High,
				Reasons: []string{"120 rows inserted into customers", "written by mysqldump", "columns holding personal data: email, phone"}, Fingerprint: "11112222333344445555666677778888"},
			{Kind: domain.KindFile, Category: domain.CategoryName, Detector: "document-author", Label: "Person name in document metadata (author)", Country: "EU",
				Location: domain.Location{Path: "docs/offer.docx", Line: 1, Column: 1}, Value: sentinel, Masked: "•••• •••••••", Confidence: domain.Medium,
				Reasons: []string{"a person's name in the document's dc:creator"}, Fingerprint: "99990000aaaabbbbccccddddeeeeffff"},
		},
		Summary: domain.Summary{FilesScanned: 7, FilesSkipped: []domain.Skipped{{Path: "docs/offer.docx", Reason: "a document format ATO does not open; only its author metadata was read", NotRead: true}}},
	}
}

func golden(t *testing.T, name string, r Report) {
	t.Helper()
	for _, format := range Formats() {
		var buf bytes.Buffer
		if err := Write(format, &buf, r, Options{}); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "golden", name+"."+format)
		if os.Getenv("ATO_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run with ATO_UPDATE_GOLDEN=1 to create it)", err)
		}
		if got := strings.ReplaceAll(buf.String(), "\r\n", "\n"); got != strings.ReplaceAll(string(want), "\r\n", "\n") {
			t.Errorf("%s differs from %s:\n%s", format, path, got)
		}
	}
}

func TestJSONContract(t *testing.T) {
	var buf bytes.Buffer
	if err := Write("json", &buf, sample(), Options{}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join("..", "..", "schemas", "findings.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Required   []string `json:"required"`
		Properties struct {
			SchemaVersion struct {
				Const string `json:"const"`
			} `json:"schema_version"`
			Findings struct {
				Items struct {
					Required []string `json:"required"`
				} `json:"items"`
			} `json:"findings"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		t.Fatal(err)
	}
	if s.Properties.SchemaVersion.Const != SchemaVersion {
		t.Fatalf("schema says %q, code says %q", s.Properties.SchemaVersion.Const, SchemaVersion)
	}
	for _, k := range s.Required {
		if _, ok := doc[k]; !ok {
			t.Errorf("report misses %q", k)
		}
	}
	first := doc["findings"].([]any)[0].(map[string]any)
	for _, k := range s.Properties.Findings.Items.Required {
		if _, ok := first[k]; !ok {
			t.Errorf("finding misses %q", k)
		}
	}
	if _, ok := first["value"]; ok {
		t.Error("a finding must never carry a value field")
	}
}

func TestSARIFLevels(t *testing.T) {
	var buf bytes.Buffer
	if err := Write("sarif", &buf, sample(), Options{}); err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	res := log.Runs[0].Results
	if log.Version != "2.1.0" || len(res) != 4 || res[0].Level != "error" || res[1].Level != "note" || res[2].Level != "warning" || res[3].Level != "warning" || len(log.Runs[0].Tool.Driver.Rules) != 3 {
		t.Fatalf("unexpected SARIF: %+v", log)
	}
}

func TestUnknownFormat(t *testing.T) {
	if err := Write("xml", &bytes.Buffer{}, sample(), Options{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestInventoryFormats(t *testing.T) {
	inv := Inventory{Tool: "ato", Version: "test", Files: 1, Entries: []domain.InventoryEntry{{
		Field:          domain.Field{Name: "diagnosis", Container: "patients", Format: "sql", Location: domain.Location{Path: "db/001.sql", Line: 3, Column: 3}},
		Classification: domain.Classification{Category: domain.CategorySpecial, Special: domain.SpecialHealth, Matched: "diagnosis"},
	}}}
	var csvOut, jsonOut bytes.Buffer
	if err := WriteInventory("csv", &csvOut, inv); err != nil {
		t.Fatal(err)
	}
	if want := "Field,Table or model,Category of personal data,Special category (Art. 9),Source format,Source file,Line\ndiagnosis,patients,special-category,health,sql,db/001.sql,3\n"; csvOut.String() != want {
		t.Fatalf("csv:\n%s", csvOut.String())
	}
	if err := WriteInventory("json", &jsonOut, inv); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &doc); err != nil || doc["schema_version"] != InventorySchemaVersion {
		t.Fatalf("json: %v %s", err, jsonOut.String())
	}
	if WriteInventory("xml", &bytes.Buffer{}, inv) == nil {
		t.Fatal("unknown format accepted")
	}
}

func TestHTMLIsSelfContained(t *testing.T) {
	var findings, inventory bytes.Buffer
	if err := Write("html", &findings, sample(), Options{}); err != nil {
		t.Fatal(err)
	}
	inv := Inventory{Tool: "ato", Version: "test", Files: 3, Entries: []domain.InventoryEntry{
		{Field: domain.Field{Name: "diagnosis", Container: "patients", Location: domain.Location{Path: "db/001.sql", Line: 4}}, Classification: domain.Classification{Category: domain.CategorySpecial, Special: domain.SpecialHealth}},
		{Field: domain.Field{Name: "<script>", Container: "patients", Location: domain.Location{Path: "db/001.sql", Line: 5}}, Classification: domain.Classification{Category: domain.CategoryName}},
	}}
	if err := WriteInventory("html", &inventory, inv); err != nil {
		t.Fatal(err)
	}
	for name, page := range map[string]string{"findings": findings.String(), "inventory": inventory.String()} {
		for _, bad := range []string{"http://", "https://", "<script", "src="} {
			if strings.Contains(page, bad) {
				t.Errorf("%s page contains %q: it must load nothing and run nothing", name, bad)
			}
		}
	}
	if !strings.Contains(inventory.String(), "health (Art. 9)") {
		t.Error("inventory page should name the special category")
	}
}
