package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/walk"
)

// The names, identifiers and dates below are made up.

func scanRecords(t *testing.T, files map[string]string, records []string) []domain.Finding {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Scan(Request{Root: dir, Walk: walk.Options{NoGit: true}, Records: records, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	return res.Findings
}

const pid = "PID|1||48213^^^HOSP^MR||Varnell^Odile||19750412|F\n"

func TestRecordsAreReportedFromFilesOfEveryKind(t *testing.T) {
	fs := scanRecords(t, map[string]string{
		"feed/admit.hl7":    pid,
		"export/genome.csv": "rsid,chromosome,position,genotype\n" + strings.Repeat("rs1000001,1,50000,AG\n", 5),
		"app/notes.txt":     "nothing here\n",
	}, RecordDetectors())
	if len(fs) != 2 || fs[0].Detector != "rsid-genotypes" || fs[1].Detector != "hl7-patient" {
		t.Fatalf("got %+v", fs)
	}
	for _, f := range fs {
		if f.Kind != domain.KindRecord || f.Confidence != domain.High || f.Fingerprint == "" || f.TestPath {
			t.Errorf("%+v", f)
		}
	}
}

func TestRecordsInTestPathsAreLoweredOneStep(t *testing.T) {
	fs := scanRecords(t, map[string]string{"tests/fixtures/admit.hl7": pid}, RecordDetectors())
	if len(fs) != 1 || fs[0].Confidence != domain.Medium || !fs[0].TestPath {
		t.Fatalf("got %+v", fs)
	}
}

func TestRecordsHonourIgnoreComments(t *testing.T) {
	fs := scanRecords(t, map[string]string{"feed/admit.txt": "# ato:ignore synthetic admission for the parser test\n" + pid}, RecordDetectors())
	if len(fs) != 1 || !fs[0].Ignored {
		t.Fatalf("got %+v", fs)
	}
}

func TestRecordFingerprintsIgnoreTheValues(t *testing.T) {
	a := scanRecords(t, map[string]string{"feed/admit.hl7": pid}, RecordDetectors())
	b := scanRecords(t, map[string]string{"feed/admit.hl7": strings.Replace(pid, "Varnell", "Brannock", 1)}, RecordDetectors())
	if len(a) != 1 || len(b) != 1 || a[0].Fingerprint != b[0].Fingerprint {
		t.Fatalf("fingerprints differ: %+v %+v", a, b)
	}
}

func TestRecordChecksCanBeSwitchedOff(t *testing.T) {
	if fs := scanRecords(t, map[string]string{"feed/admit.hl7": pid}, []string{"fhir-patient"}); len(fs) != 0 {
		t.Fatalf("got %+v", fs)
	}
}
