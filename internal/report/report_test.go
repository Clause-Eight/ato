package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// sentinel is a synthetic value used only to prove that reports never print raw values.
const sentinel = "SENTINEL-4929000000006"

func sample() Report {
	return Report{
		Tool: "pii-eu", Version: "test", Root: ".",
		Findings: []domain.Finding{
			{Detector: "card", Label: "Payment card number", Country: "EU", Location: domain.Location{Path: "app/config.yml", Line: 3, Column: 9},
				Value: sentinel, Masked: "••••••••••••0006", Confidence: domain.High, Reasons: []string{"brand prefix, length and Luhn check are valid", `the word "card" is nearby`}, Fingerprint: "0123456789abcdef0123456789abcdef"},
			{Detector: "ro-cnp", Label: "Romanian personal number (CNP)", Country: "RO", Location: domain.Location{Path: "tests/fixtures/users.json", Line: 14, Column: 12},
				Value: sentinel, Masked: "•••••••••••••", Confidence: domain.Low, Reasons: []string{"date, county code and check digit are valid", "in a test, fixture or example path"}, TestPath: true, Fingerprint: "fedcba9876543210fedcba9876543210"},
		},
		Summary: domain.Summary{FilesScanned: 42, FilesSkipped: []domain.Skipped{{Path: "logo.png", Reason: "binary"}}},
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
	for _, format := range Formats() {
		var buf bytes.Buffer
		if err := Write(format, &buf, sample(), Options{}); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "golden", "report."+format)
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
	if log.Version != "2.1.0" || len(res) != 2 || res[0].Level != "error" || res[1].Level != "note" || len(log.Runs[0].Tool.Driver.Rules) != 2 {
		t.Fatalf("unexpected SARIF: %+v", log)
	}
}

func TestUnknownFormat(t *testing.T) {
	if err := Write("xml", &bytes.Buffer{}, sample(), Options{}); err == nil {
		t.Fatal("expected an error")
	}
}
