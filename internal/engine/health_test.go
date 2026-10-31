package engine

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/synth"
)

func scanAll(t *testing.T, path, text string) []domain.Finding {
	t.Helper()
	fs, err := ScanReader(path, strings.NewReader(text), detectors.All())
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func only(fs []domain.Finding, detector string) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Detector == detector {
			out = append(out, f)
		}
	}
	return out
}

func TestCodesInARecordAboutAPerson(t *testing.T) {
	cnp, _ := synth.For("ro-cnp")
	cases := []struct {
		name, path, text, detector string
	}{
		{"csv row with a patient and a diagnosis column", "data/visits.csv", "patient_id,first_name,diagnosis\n17,Ana,E11.9\n", "icd10"},
		{"json line with a patient key", "app/records.json", `{"patient_id": 42, "icd10": "J45.9"}` + "\n", "icd10"},
		{"personal field on the line before", "app/seed.yml", "- first_name: Ana\n  diagnosis: E11\n", "icd10"},
		{"national number on the same line", "app/export.txt", "cnp " + cnp(rand.New(rand.NewSource(3))) + " diagnosis E11.9\n", "icd10"},
		{"icd-11 stem code", "app/records.json", `{"patient_id": 42, "diagnosis": "5A11"}` + "\n", "icd11"},
		{"atc code of a medication", "app/records.json", `{"patient": 7, "medication": "A10BA02"}` + "\n", "atc-code"},
		{"aic code of a medicine", "app/ricette.json", `{"paziente": 7, "farmaco": "012345676"}` + "\n", "it-aic"},
	}
	for _, c := range cases {
		fs := only(scanAll(t, c.path, c.text), c.detector)
		if len(fs) != 1 {
			t.Errorf("%s: got %d %s findings", c.name, len(fs), c.detector)
			continue
		}
		f := fs[0]
		if f.Special != domain.SpecialHealth || f.Category != domain.CategorySpecial || f.Confidence < domain.Medium {
			t.Errorf("%s: %+v", c.name, f)
		}
		if strings.Contains(f.Masked, f.Value) {
			t.Errorf("%s: the mask shows the value", c.name)
		}
		if !strings.Contains(strings.Join(f.Reasons, "; "), "in a record about a person") {
			t.Errorf("%s: reasons %v", c.name, f.Reasons)
		}
	}
}

func TestCodesWithoutAPersonAreNotReported(t *testing.T) {
	iban, _ := synth.For("iban")
	r := rand.New(rand.NewSource(5))
	cases := []struct{ name, path, text string }{
		{"diagnosis code alone", "app/codes.yml", "diagnosis: E11.9\n"},
		{"seat next to a name", "app/booking.json", `{"first_name": "Ana", "seat": "B12"}` + "\n"},
		{"version next to a name", "app/user.yml", "first_name: Ana\nversion: E11\n"},
		{"code list in a non-medical csv", "data/parts.csv", "part,code,first_name\nbolt,A10BA02,Ana\n"},
		{"medication catalogue without a person", "data/drugs.csv", "drug,atc\nexample,A10BA02\n"},
		{"iban next to a patient key", "app/pay.yml", "patient_id: 1\nvalue = \"" + iban(r) + "\"\n"},
		{"aic without a medicine word", "app/ids.json", `{"patient_id": 7, "ref": "012345676"}` + "\n"},
		{"room next to a patient", "app/ward.json", `{"patient_id": 7, "room": "B12"}` + "\n"},
		{"svg offset next to a name", "app/font.svg", `<text dx="E11" first_name="Ana">` + "\n"},
		{"medical word far from the code", "app/min.json", `{"patient_id": 7, "diagnosis": "x", "` + strings.Repeat("k", 80) + `": "E11"}` + "\n"},
		{"person far from the code on a long line", "app/min.json", `{"first_name": "Ana", "` + strings.Repeat("k", 400) + `": 1, "diagnosis": "E11"}` + "\n"},
	}
	for _, c := range cases {
		for _, f := range scanAll(t, c.path, c.text) {
			if f.Special != "" {
				t.Errorf("%s: reported %s", c.name, f.Detector)
			}
		}
	}
}

func TestCodeDetectorsKeepTheLineFlat(t *testing.T) {
	// A long code-like line with many letter-digit pairs and no person must not report anything special.
	line := strings.Repeat("A01 B02 C03 E11.9 5A11 A10BA02 ", 50) + "diagnosis\n"
	for _, f := range scanAll(t, "app/notes.txt", line) {
		if f.Special != "" {
			t.Fatalf("reported %s", f.Detector)
		}
	}
}
