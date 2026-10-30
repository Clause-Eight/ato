package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/domain"
)

// validate checks a document against the subset of JSON Schema the published schemas use: type, required,
// properties, additionalProperties, items, enum, const, minimum and local $ref.
func validate(t *testing.T, schema, root map[string]any, doc any, at string) {
	t.Helper()
	if ref, ok := schema["$ref"].(string); ok {
		def := root
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			def = def[part].(map[string]any)
		}
		validate(t, def, root, doc, at)
		return
	}
	if c, ok := schema["const"]; ok && doc != c {
		t.Errorf("%s: %v, schema wants %v", at, doc, c)
	}
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			found = found || e == doc
		}
		if !found {
			t.Errorf("%s: %v is not one of %v", at, doc, enum)
		}
	}
	switch v := doc.(type) {
	case map[string]any:
		if typ, ok := schema["type"]; ok && typ != "object" {
			t.Errorf("%s: object where the schema wants %v", at, typ)
		}
		for _, r := range asStrings(schema["required"]) {
			if _, ok := v[r]; !ok {
				t.Errorf("%s: missing required %q", at, r)
			}
		}
		props, _ := schema["properties"].(map[string]any)
		for k, x := range v {
			p, ok := props[k].(map[string]any)
			if !ok {
				if schema["additionalProperties"] == false {
					t.Errorf("%s: %q is not in the schema", at, k)
				}
				continue
			}
			validate(t, p, root, x, at+"."+k)
		}
	case []any:
		items, _ := schema["items"].(map[string]any)
		for i, x := range v {
			validate(t, items, root, x, fmt.Sprintf("%s[%d]", at, i))
		}
	case float64:
		if typ := schema["type"]; typ != nil && typ != "integer" && typ != "number" {
			t.Errorf("%s: number where the schema wants %v", at, typ)
		}
		if min, ok := schema["minimum"].(float64); ok && v < min {
			t.Errorf("%s: %v below %v", at, v, min)
		}
	case string:
		if typ := schema["type"]; typ != nil && typ != "string" {
			t.Errorf("%s: string where the schema wants %v", at, typ)
		}
	case bool:
		if typ := schema["type"]; typ != nil && typ != "boolean" {
			t.Errorf("%s: boolean where the schema wants %v", at, typ)
		}
	}
}

func asStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func loadJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func withField(r Report) Report {
	r.Findings = append(r.Findings, domain.Finding{
		Kind: domain.KindField, Category: domain.CategorySpecial, Special: domain.SpecialHealth, Detector: "special-category",
		Label: "Field holding health data (GDPR Art. 9)", Country: "EU", Location: domain.Location{Path: "db/001.sql", Line: 4, Column: 3},
		Masked: "patients.diagnosis", Confidence: domain.Medium, Reasons: []string{`the name contains "diagnosis"`}, Fingerprint: "00112233445566778899aabbccddeeff",
	})
	return r
}

// syntheticStore is a made-up Terraform data store.
func syntheticStore() domain.Store {
	return domain.Store{
		Type: "aws_s3_bucket", Name: "exports", Provider: "aws", Service: "object-storage", Names: []string{"example-exports"},
		Regions: []string{"us-east-1"}, InEEA: domain.Setting{State: domain.No, Why: "region us-east-1 is outside the EEA"},
		Encryption: domain.Setting{State: domain.Yes}, Retention: domain.Setting{State: domain.No}, Backup: domain.Setting{State: domain.Unknown},
		Public: domain.Setting{State: domain.No}, PersonalData: true, PersonalWhy: `tagged data_classification with "pii"`,
		Location: domain.Location{Path: "infra/main.tf", Line: 7, Column: 1},
	}
}

// syntheticModule is a made-up module call that pii-eu does not read inside.
func syntheticModule() domain.Store {
	unknown := domain.Setting{State: domain.Unknown, Why: "decided inside the module"}
	return domain.Store{
		Type: domain.ModuleType, Name: "orders_db", Provider: "unknown", Service: "unknown", InEEA: unknown,
		Encryption: unknown, Retention: unknown, Backup: unknown, Public: unknown,
		Module: "git::https://example.com/acme/modules.git//thing", ModuleSettings: []string{"storage_encrypted"},
		Location: domain.Location{Path: "infra/main.tf", Line: 20, Column: 1},
	}
}

func withStore(r Report) Report {
	s := syntheticStore()
	r.Findings = append(r.Findings, domain.Finding{
		Kind: domain.KindInfra, Detector: "tf-non-eea-region", Label: "Data store outside the EEA", Country: "EU",
		Location: s.Location, Masked: s.Address(), Confidence: domain.High, Reasons: []string{s.InEEA.Why, "linked to personal data: " + s.PersonalWhy},
		Fingerprint: "ffeeddccbbaa99887766554433221100", Store: &s,
	})
	return r
}

func TestReportsMatchThePublishedSchemas(t *testing.T) {
	var buf bytes.Buffer
	if err := Write("json", &buf, withSink(withStore(withField(sample()))), Options{}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	schema := loadJSON(t, filepath.Join("..", "..", "schemas", "findings.schema.json"))
	validate(t, schema, schema, doc, "report")

	buf.Reset()
	inv := Inventory{Tool: "pii-eu", Version: "test", Files: 1, Entries: []domain.InventoryEntry{{
		Field:          domain.Field{Name: "diagnosis", Container: "patients", Format: "sql", Location: domain.Location{Path: "db/001.sql", Line: 4, Column: 3}},
		Classification: domain.Classification{Category: domain.CategorySpecial, Special: domain.SpecialHealth, Matched: "diagnosis"},
	}, {
		Field:          domain.Field{Name: "email", Container: "User", Format: "typeorm", Location: domain.Location{Path: "src/user.entity.ts", Line: 9, Column: 5}},
		Classification: domain.Classification{Category: domain.CategoryContact, Matched: "email"},
	}}, Stores: []domain.Store{syntheticStore(), syntheticModule()}}
	if err := WriteInventory("json", &buf, inv); err != nil {
		t.Fatal(err)
	}
	var invDoc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &invDoc); err != nil {
		t.Fatal(err)
	}
	invSchema := loadJSON(t, filepath.Join("..", "..", "schemas", "inventory.schema.json"))
	validate(t, invSchema, invSchema, invDoc, "inventory")
	if invDoc["schema_version"] != InventorySchemaVersion {
		t.Errorf("inventory schema version %v", invDoc["schema_version"])
	}
	stores := invDoc["stores"].([]any)
	if _, ok := stores[0].(map[string]any)["module"]; ok {
		t.Error("a resource has no module object")
	}
	if m, ok := stores[1].(map[string]any)["module"].(map[string]any); !ok || m["source"] == "" || len(m["settings"].([]any)) != 1 {
		t.Errorf("module call: %v", stores[1])
	}
}

// frozen are the JSON fields promised stable for every 1.x release (docs/COMPATIBILITY.md). A 1.x release may
// add fields, never remove or rename these.
var frozen = map[string][]string{
	"report":  {"schema_version", "tool", "summary", "findings"},
	"summary": {"findings", "files_scanned", "files_skipped", "unreadable", "ignored", "baselined"},
	"finding": {"kind", "detector", "label", "country", "path", "line", "column", "confidence", "masked", "reasons", "sample", "test_path", "fingerprint"},
}

func TestFrozenFieldsAreStillWritten(t *testing.T) {
	var buf bytes.Buffer
	if err := Write("json", &buf, sample(), Options{}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	objects := map[string]map[string]any{"report": doc, "summary": doc["summary"].(map[string]any), "finding": doc["findings"].([]any)[0].(map[string]any)}
	for name, fields := range frozen {
		for _, f := range fields {
			if _, ok := objects[name][f]; !ok {
				t.Errorf("%s no longer has %q, which 1.x promises", name, f)
			}
		}
	}
	if !strings.HasPrefix(SchemaVersion, "1.") {
		t.Errorf("schema version %s: a new major version needs a new compatibility promise", SchemaVersion)
	}
	keys := make([]string, 0, len(frozen))
	for k := range frozen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
}

func TestInfraFindingsInEachFormat(t *testing.T) {
	r := withStore(Report{Tool: "pii-eu", Version: "test"})
	var buf bytes.Buffer
	if err := Write("sarif", &buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 1 || rules[0].ID != "tf-non-eea-region" || !strings.Contains(rules[0].FullDescription.Text, "Chapter V") {
		t.Fatalf("rule: %+v", rules)
	}
	buf.Reset()
	if err := Write("json", &buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind": "infra"`, `"masked": "aws_s3_bucket.exports"`, `"provider": "aws"`, `"personal_data": true`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("json misses %s:\n%s", want, buf.String())
		}
	}
}

func withSink(r Report) Report {
	r.Findings = append(r.Findings, domain.Finding{
		Kind: domain.KindSink, Category: domain.CategoryContact, Sink: domain.SinkLog, Detector: "log-sink",
		Label: "Personal data written to a log", Country: "EU", Location: domain.Location{Path: "app/handler.go", Line: 12, Column: 30},
		Masked: "user.Email", Confidence: domain.Medium, Reasons: []string{`the name contains "email"`, "passed to log.Printf, which writes to a log"}, Fingerprint: "ffeeddccbbaa99887766554433221100",
	})
	return r
}
