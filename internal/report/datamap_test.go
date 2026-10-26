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

func TestReadMapRejectsOtherDocuments(t *testing.T) {
	for _, doc := range []string{`not json`, `{"schema_version": "2.0", "components": []}`} {
		if _, err := ReadMap(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: want an error", doc)
		}
	}
}
