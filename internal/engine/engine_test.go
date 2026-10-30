package engine

import (
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/domain"
	"github.com/Clause-Eight/ato/internal/synth"
	"github.com/Clause-Eight/ato/internal/walk"
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
	if !fs[0].TestPath || fs[0].Confidence != domain.Medium || !strings.Contains(strings.Join(fs[0].Reasons, ";"), "test, fixture") {
		t.Fatalf("a validated value in a test path keeps its confidence and says where it is: %+v", fs[0])
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
	fs := scan(t, "a.go", "x := 1234567 // ato:ignore synthetic test value\n")
	if len(fs) != 1 || !fs[0].Ignored {
		t.Fatalf("same-line ignore: %+v", fs)
	}
	fs = scan(t, "a.py", "# ato:ignore: documented example\nx = 1234567\n")
	if len(fs) != 1 || !fs[0].Ignored {
		t.Fatalf("previous-line ignore: %+v", fs)
	}
	fs = scan(t, "a.html", "<!-- ato:ignore -->\n1234567\n")
	if len(fs) != 1 || fs[0].Ignored || !strings.Contains(strings.Join(fs[0].Reasons, ";"), "no reason") {
		t.Fatalf("an ignore without a reason must not apply: %+v", fs)
	}
	fs = scan(t, "a.go", "x := 1234567 // pii-eu:ignore synthetic test value\n")
	if len(fs) != 1 || !fs[0].Ignored || !fs[0].LegacyIgnore {
		t.Fatalf("the deprecated pii-eu:ignore form: %+v", fs)
	}
	if fs = scan(t, "a.go", "x := 1234567 // ato:ignore synthetic test value\n"); fs[0].LegacyIgnore {
		t.Fatal("ato:ignore is not the legacy form")
	}
	fs = scan(t, "a.go", "x := 1234567 // potato:ignore synthetic test value\n")
	if len(fs) != 1 || fs[0].Ignored {
		t.Fatalf("ato:ignore must start a word: %+v", fs)
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

// TestRecallInRecordsAndTestPaths: a validated value in a test file is not lowered for its path, and a value next
// to another personal field (a name, a birth date, an address) is raised as part of a record about a person (#269).
func TestRecallInRecordsAndTestPaths(t *testing.T) {
	pesel, ok := detectors.Get("pl-pesel")
	if !ok {
		t.Fatal("no pl-pesel detector")
	}
	gen, ok := synth.For("pl-pesel")
	if !ok {
		t.Fatal("no pl-pesel generator")
	}
	v := gen(rand.New(rand.NewSource(3)))
	conf := func(path, text string) domain.Confidence {
		t.Helper()
		fs, err := ScanReader(path, strings.NewReader(text), []detectors.Detector{pesel})
		if err != nil {
			t.Fatal(err)
		}
		if len(fs) != 1 {
			t.Fatalf("%s %q: %d findings", path, text, len(fs))
		}
		return fs[0].Confidence
	}
	base := conf("app/data.json", `{ref: `+v+`}`+"\n")
	if got := conf("tests/fixtures/data.json", `{ref: `+v+`}`+"\n"); got != base {
		t.Fatalf("a test path changed the confidence: %s, outside tests %s", got, base)
	}
	if got := conf("tests/fixtures/people.json", `{first_name: "Ana", ref: `+v+`}`+"\n"); got <= base {
		t.Fatalf("a first_name in the same record should raise the value: %s, alone %s", got, base)
	}
	if got := conf("app/people.yaml", "  last_name: Nowak\n  ref: "+v+"\n"); got <= base {
		t.Fatalf("a last_name on the line before should raise the value: %s, alone %s", got, base)
	}
	if got := conf("app/data.json", `{firstname_count: 3, ref: `+v+`}`+"\n"); got != base {
		t.Fatalf("a name-like word that is not a personal field must not raise: %s", got)
	}
}

// TestFalseAlarmsFromOpenSourceApps: shapes found by scanning public open-source applications (#271).
func TestFalseAlarmsFromOpenSourceApps(t *testing.T) {
	scanWith := func(name, path, text string) []domain.Finding {
		t.Helper()
		d, ok := detectors.Get(name)
		if !ok {
			t.Fatalf("no %s detector", name)
		}
		fs, err := ScanReader(path, strings.NewReader(text+"\n"), []detectors.Detector{d})
		if err != nil {
			t.Fatal(err)
		}
		var reported []domain.Finding
		for _, f := range fs {
			if f.Confidence >= domain.Medium {
				reported = append(reported, f)
			}
		}
		return reported
	}
	gen := func(name string) string {
		g, ok := synth.For(name)
		if !ok {
			t.Fatalf("no generator for %s", name)
		}
		return g(rand.New(rand.NewSource(5)))
	}
	card := gen("card")
	if fs := scanWith("card", "specs/openapi.yaml", `  "height": `+card+`,`); len(fs) != 0 {
		t.Fatalf("a number under height is not a card: %v", fs[0].Reasons)
	}
	if fs := scanWith("card", "app/pay.json", `  "pan": "`+card+`",`); len(fs) != 1 {
		t.Fatal("a card under its own key must still be reported")
	}
	nif := gen("pt-nif")
	if fs := scanWith("pt-nif", "tests/customer.spec.ts", `{ postal_code: "1000", phone: "`+nif+`" }`); len(fs) != 0 {
		t.Fatalf("a number under phone is a phone number: %v", fs[0].Reasons)
	}
	if fs := scanWith("pt-nif", "docs/page.json", `{\"phone\": \"`+nif+`\"}`); len(fs) != 0 {
		t.Fatalf("an escaped phone key still names the value: %v", fs[0].Reasons)
	}
	line := `'NUMER' = '40FBD218-9DD0-48F1-81B0-123456789012';`
	ctx := lineContext{line: line}
	at := strings.Index(line, "123456789012")
	if !ctx.inUUID(at, at+12) || ctx.inUUID(0, 5) {
		t.Fatal("a candidate inside a UUID is rejected, one outside is not")
	}
	email, _ := detectors.Get("email")
	for _, addr := range []string{"jane@test.com", "ion@email.com", "x@sub.example.com"} {
		if email.Validate(addr) {
			t.Errorf("%s is on a placeholder domain", addr)
		}
	}
}

// TestWholeQuotedDistinctIdentifiers: a quoted value that is all of a distinct identifier (a date and a check digit,
// or a check letter) is raised; a number with a plain checksum is not, nor is an unquoted one (#274).
func TestWholeQuotedDistinctIdentifiers(t *testing.T) {
	conf := func(name, text string) domain.Confidence {
		t.Helper()
		d, _ := detectors.Get(name)
		fs, err := ScanReader("app/data.json", strings.NewReader(text+"\n"), []detectors.Detector{d})
		if err != nil {
			t.Fatal(err)
		}
		if len(fs) != 1 {
			t.Fatalf("%s %q: %d findings", name, text, len(fs))
		}
		return fs[0].Confidence
	}
	gen := func(name string) string {
		g, _ := synth.For(name)
		return g(rand.New(rand.NewSource(11)))
	}
	pesel := gen("pl-pesel")
	if quoted, bare := conf("pl-pesel", `{"ref": "`+pesel+`"}`), conf("pl-pesel", `ref = `+pesel); quoted <= bare {
		t.Fatalf("a whole quoted PESEL should be raised: quoted %s, bare %s", quoted, bare)
	}
	if inside, bare := conf("pl-pesel", `{"ref": "no `+pesel+`"}`), conf("pl-pesel", `ref = `+pesel); inside != bare {
		t.Fatalf("a PESEL inside a longer quoted string is not the whole value: %s, bare %s", inside, bare)
	}
	taj := gen("hu-taj")
	if quoted, bare := conf("hu-taj", `{"ref": "`+taj+`"}`), conf("hu-taj", `ref = `+taj); quoted != bare {
		t.Fatalf("a plain-checksum number is not raised for being quoted: quoted %s, bare %s", quoted, bare)
	}
}
