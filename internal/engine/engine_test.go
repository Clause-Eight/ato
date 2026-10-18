package engine

import (
	"os"
	"path/filepath"
	"regexp"
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
