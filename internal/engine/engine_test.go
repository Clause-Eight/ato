package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/walk"
)

// sevens is a fake detector: seven-digit numbers that end in 7.
var sevens = detectors.Detector{
	Name: "sevens", Label: "Test number", Country: "XX", OnByDefault: true,
	Pattern:    regexp.MustCompile(`\d{7}`),
	Normalize:  func(s string) string { return s },
	Validate:   func(s string) bool { return strings.HasSuffix(s, "7") },
	Mask:       detectors.MaskKeepLast(2),
	Keywords:   []string{"secret"},
	Base:       domain.Medium,
	Samples:    map[string]bool{"1111117": true},
	Validation: "ends in 7",
}

func scan(t *testing.T, path, text string) []domain.Finding {
	t.Helper()
	fs, err := ScanReader(path, strings.NewReader(text), []detectors.Detector{sevens})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestValidatedCandidatesOnly(t *testing.T) {
	fs := scan(t, "a.txt", "id 1234567 and 1234568\n")
	if len(fs) != 1 || fs[0].Value != "1234567" || fs[0].Location != (domain.Location{Path: "a.txt", Line: 1, Column: 4}) {
		t.Fatalf("got %+v", fs)
	}
	if fs[0].Masked != "•••••67" || fs[0].Confidence != domain.Medium {
		t.Fatalf("mask or confidence wrong: %+v", fs[0])
	}
}

func TestBoundariesRejectLongerTokens(t *testing.T) {
	if fs := scan(t, "a.txt", "x1234567 12345677 ID1234567\n"); len(fs) != 0 {
		t.Fatalf("expected none, got %+v", fs)
	}
	if fs := scan(t, "a.txt", "x = 3.1234567 and 1234567.5 and 1,1234567 and {-1234567, -1234567}\n"); len(fs) != 0 {
		t.Fatalf("digits of a decimal number are not values: %+v", fs)
	}
	if fs := scan(t, "a.txt", "end of sentence 1234567. Next, 1234567, done\n"); len(fs) != 2 {
		t.Fatalf("a full stop or comma after a value is a boundary: %+v", fs)
	}
	if fs := scan(t, "a.txt", `"1234567",(1234567)`+"\n"); len(fs) != 2 {
		t.Fatalf("punctuation is a boundary: %+v", fs)
	}
}

func TestKeywordOnLineOrLineBeforeRaisesConfidence(t *testing.T) {
	fs := scan(t, "a.txt", "# secret below\nvalue=1234567\n")
	if len(fs) != 1 || fs[0].Confidence != domain.High || fs[0].Location.Line != 2 {
		t.Fatalf("got %+v", fs)
	}
}

func TestSamplesAndTestPaths(t *testing.T) {
	fs := scan(t, "a.txt", "1111117\n")
	if !fs[0].Sample || fs[0].Confidence != domain.Low {
		t.Fatalf("sample not marked: %+v", fs[0])
	}
	fs = scan(t, "tests/fixtures/users.json", "1234567\n")
	if !fs[0].TestPath || fs[0].Confidence != domain.Low {
		t.Fatalf("test path not lowered: %+v", fs[0])
	}
}

func TestIsTestPath(t *testing.T) {
	yes := []string{"tests/a.go", "src/__tests__/x.js", "pkg/user_test.go", "app/user.spec.ts", "test_users.py", "examples/demo/x.json", "testdata/in.txt"}
	no := []string{"src/contest/a.go", "latest/x.go", "src/users.go", "attestation.md"}
	for _, p := range yes {
		if !IsTestPath(p) {
			t.Errorf("%s should be a test path", p)
		}
	}
	for _, p := range no {
		if IsTestPath(p) {
			t.Errorf("%s should not be a test path", p)
		}
	}
}

func TestFingerprintIgnoresTheValue(t *testing.T) {
	a := scan(t, "a.txt", "card=1234567\n")
	b := scan(t, "a.txt", "card=7654327\n")
	if a[0].Fingerprint != b[0].Fingerprint {
		t.Fatal("fingerprint changed with the value")
	}
	c := scan(t, "b.txt", "card=1234567\n")
	if a[0].Fingerprint == c[0].Fingerprint {
		t.Fatal("fingerprint should depend on the path")
	}
	if strings.Contains(a[0].Fingerprint, "1234567") {
		t.Fatal("fingerprint contains the value")
	}
}

func TestDedupeKeepsMostConfident(t *testing.T) {
	wide := sevens
	wide.Name, wide.Base, wide.Pattern = "wide", domain.Low, regexp.MustCompile(`\d{3} ?\d{4}`)
	fs, err := ScanReader("a.txt", strings.NewReader("1234567\n"), []detectors.Detector{wide, sevens})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Detector != "sevens" {
		t.Fatalf("got %+v", fs)
	}
}

func TestScanFolderSkipsBinaryAndBigFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", []byte("1234567\n"))
	write("b.bin", []byte("1234567\x00\x01"))
	write("big.txt", []byte(strings.Repeat("1234567\n", 100)))
	write("sub/c.txt", []byte("no digits here\n"))
	res, err := Scan(Request{Root: dir, Walk: walk.Options{MaxFileSize: 100, NoGit: true}, Detectors: []detectors.Detector{sevens}, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Location.Path != "a.txt" {
		t.Fatalf("findings: %+v", res.Findings)
	}
	if res.Summary.FilesScanned != 2 || len(res.Summary.FilesSkipped) != 2 {
		t.Fatalf("summary: %+v", res.Summary)
	}
}

func TestVeryLongLinesDoNotFail(t *testing.T) {
	line := strings.Repeat("a", 3<<20) + " 1234567\n" + "1234567\n"
	fs := scan(t, "a.txt", line)
	if len(fs) != 1 || fs[0].Location.Line != 2 {
		t.Fatalf("got %+v", fs)
	}
}

func TestIgnoreComments(t *testing.T) {
	fs := scan(t, "a.go", "x := 1234567 // pii-eu:ignore synthetic test value\n")
	if len(fs) != 1 || !fs[0].Ignored {
		t.Fatalf("same-line ignore: %+v", fs)
	}
	fs = scan(t, "a.py", "# pii-eu:ignore: documented example\nx = 1234567\n")
	if len(fs) != 1 || !fs[0].Ignored {
		t.Fatalf("previous-line ignore: %+v", fs)
	}
	fs = scan(t, "a.html", "<!-- pii-eu:ignore -->\n1234567\n")
	if len(fs) != 1 || fs[0].Ignored || !strings.Contains(strings.Join(fs[0].Reasons, ";"), "no reason") {
		t.Fatalf("an ignore without a reason must not apply: %+v", fs)
	}
}

func TestNumberTablesAreLowered(t *testing.T) {
	fs := scan(t, "a.go", "seeds = {1234567, 9876543210, 1234567890123, 5555555555}\n")
	if len(fs) != 1 || fs[0].Confidence != domain.Low {
		t.Fatalf("got %+v", fs)
	}
}

func TestContextLowersAtMostOneStep(t *testing.T) {
	fs := scan(t, "tests/fixtures/users.json", `{"secret": 1234567, "a": 9876543210, "b": 1234567890123, "c": 5555555555}`+"\n")
	if len(fs) != 1 {
		t.Fatalf("got %+v", fs)
	}
	f := fs[0]
	if f.Confidence != domain.Medium || !f.Keyword {
		t.Fatalf("a keyword-confirmed value in a fixture table should stay at medium: %+v", f)
	}
	reasons := strings.Join(f.Reasons, "; ")
	if !strings.Contains(reasons, "line of long numbers") || !strings.Contains(reasons, "test, fixture or example path") {
		t.Fatalf("every lowering signal should still be listed: %q", reasons)
	}
}

func TestMillisecondTimestampsUnderTimeNamesAreLowered(t *testing.T) {
	cnp, ok := detectors.Get("ro-cnp")
	if !ok {
		t.Fatal("no ro-cnp detector")
	}
	for v := int64(1710706100000); v < 1710706200000; v++ {
		s := strconv.FormatInt(v, 10)
		if !cnp.Validate(s) {
			continue
		}
		for _, line := range []string{"created_at: " + s, `{"updatedAt": ` + s + `}`, "ts=" + s} {
			fs, err := ScanReader("events.yaml", strings.NewReader(line+"\n"), []detectors.Detector{cnp})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range fs {
				if f.Confidence > domain.Low {
					t.Fatalf("%q reported at %s: %v", line, f.Confidence, f.Reasons)
				}
			}
		}
		fs, err := ScanReader("people.yaml", strings.NewReader("cnp: "+s+"\n"), []detectors.Detector{cnp})
		if err != nil {
			t.Fatal(err)
		}
		if len(fs) != 1 || fs[0].Confidence < domain.Medium {
			t.Fatalf("a value labelled cnp must not be lowered as a time: %+v", fs)
		}
		return
	}
	t.Fatal("no millisecond timestamp in the range passes the CNP check")
}
