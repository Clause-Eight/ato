package cli

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/synth"
)

func generate(t *testing.T, detector string, seed int64) string {
	t.Helper()
	g, ok := synth.For(detector)
	if !ok {
		t.Fatalf("no generator for %s", detector)
	}
	return g(rand.New(rand.NewSource(seed)))
}

func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type jsonFindings struct {
	Findings []struct {
		Detector   string `json:"detector"`
		Path       string `json:"path"`
		Line       int    `json:"line"`
		Confidence string `json:"confidence"`
		Sample     bool   `json:"sample"`
	} `json:"findings"`
}

func fixScanJSON(t *testing.T, args ...string) jsonFindings {
	t.Helper()
	_, out, errOut := run(append([]string{"scan", "--no-git", "--format", "json"}, args...)...)
	var r jsonFindings
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v: %s", err, errOut)
	}
	return r
}

func TestFixReplacesFixtureValues(t *testing.T) {
	dir := t.TempDir()
	cnp := generate(t, "ro-cnp", 1)
	card := syntheticCard()
	other := generate(t, "ro-cnp", 2)
	writeFile(t, dir, "tests/fixtures/users.txt", "cnp: "+cnp+"\r\ncard: "+card+"\r\nnothing here\r\n")
	writeFile(t, dir, "tests/other_test.txt", "personal number (cnp) "+cnp+"\n")
	writeFile(t, dir, "app/config.txt", "cnp: "+other+"\n")
	before := readFile(t, dir, "tests/fixtures/users.txt")

	code, out, errOut := run("fix", "--no-git", "--dry-run", dir)
	if code != ExitClean {
		t.Fatalf("dry run: exit %d (%s)", code, errOut)
	}
	for _, v := range []string{cnp, card, other} {
		if strings.Contains(out, v) {
			t.Fatal("the dry run printed a found value")
		}
	}
	if !strings.Contains(out, "--- a/tests/fixtures/users.txt") || !strings.Contains(out, "@@ -1 +1 @@") || !strings.Contains(out, "Would replace 3 values in 2 files.") {
		t.Fatalf("unexpected dry run output:\n%s", out)
	}
	if readFile(t, dir, "tests/fixtures/users.txt") != before {
		t.Fatal("the dry run changed a file")
	}

	code, out, errOut = run("fix", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "Replaced 3 values in 2 files") || !strings.Contains(out, "--all") {
		t.Fatalf("fix: exit %d out %q err %q", code, out, errOut)
	}
	for _, v := range []string{cnp, card, other} {
		if strings.Contains(out, v) {
			t.Fatal("fix printed a found value")
		}
	}
	fixed := readFile(t, dir, "tests/fixtures/users.txt")
	if strings.Contains(fixed, cnp) || strings.Contains(fixed, card) {
		t.Fatal("values were not replaced")
	}
	if strings.Count(fixed, "\r\n") != 3 || !strings.Contains(fixed, "nothing here") {
		t.Fatalf("line endings or other lines changed: %q", fixed)
	}
	newCNP := strings.TrimSuffix(strings.TrimPrefix(strings.Split(fixed, "\r\n")[0], "cnp: "), "\r")
	if len(newCNP) != len(cnp) || !strings.Contains(readFile(t, dir, "tests/other_test.txt"), newCNP) {
		t.Fatal("the same value should get the same replacement, of the same length, in every file")
	}
	if !strings.Contains(readFile(t, dir, "app/config.txt"), other) {
		t.Fatal("a value outside test paths was replaced without --all")
	}

	for _, f := range fixScanJSON(t, "--min-confidence", "low", dir).Findings {
		if strings.HasPrefix(f.Path, "tests/") && (f.Confidence != "low" || !f.Sample) {
			t.Errorf("after fixing, %s:%d is reported at %s (sample %v)", f.Path, f.Line, f.Confidence, f.Sample)
		}
	}
	if _, out, _ := run("fix", "--no-git", filepath.Join(dir, "tests")); !strings.Contains(out, "Nothing to fix.") {
		t.Fatalf("a second fix should have nothing to do: %q", out)
	}

	if code, out, _ := run("fix", "--no-git", "--all", dir); code != ExitClean || !strings.Contains(out, "app/config.txt: replaced 1 value") {
		t.Fatalf("--all: %d %q", code, out)
	}
}

func TestFixUsage(t *testing.T) {
	for _, args := range [][]string{{"fix", "-"}, {"fix", "--staged", "."}, {"fix", "--format", "json", "."}, {"fix", "a", "b"}} {
		if code, _, _ := run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, ExitUsage)
		}
	}
}

// TestFixEveryGenerator fixes one value of every detector that has a generator, then checks that a re-scan
// reports none of them above low confidence.
func TestFixEveryGenerator(t *testing.T) {
	dir := t.TempDir()
	var cfg, body strings.Builder
	cfg.WriteString("[detectors]\n")
	var names []string
	for _, name := range synth.Names() {
		d, ok := detectors.Get(name)
		if !ok {
			continue
		}
		names = append(names, name)
		cfg.WriteString(`"` + name + "\" = true\n")
		word := name
		if len(d.Keywords) > 0 {
			word = d.Keywords[0]
		}
		body.WriteString(word + ": " + generate(t, name, 11) + "\n\n")
	}
	writeFile(t, dir, ".pii-eu.toml", cfg.String())
	writeFile(t, dir, "fixtures/all.txt", body.String())
	code, out, errOut := run("fix", "--no-git", "--min-confidence", "low", dir)
	if code != ExitClean {
		t.Fatalf("fix: exit %d (%s)", code, errOut)
	}
	t.Log(out)
	for _, f := range fixScanJSON(t, "--min-confidence", "medium", dir).Findings {
		t.Errorf("after fixing, %s line %d (%s) is reported at %s", f.Path, f.Line, f.Detector, f.Confidence)
	}
	if len(names) < 50 {
		t.Fatalf("only %d detectors have generators", len(names))
	}

	// Every replacement keeps the format of the value it replaced: length, separators, case and the positions of
	// letters and digits. A Base58 wallet varies in length and a street address follows its street name.
	before := strings.Split(body.String(), "\n")
	after := strings.Split(readFile(t, dir, "fixtures/all.txt"), "\n")
	if len(before) != len(after) {
		t.Fatalf("fix changed the number of lines")
	}
	for i, name := range names {
		if name == "crypto-wallet" || name == "street-address" {
			continue
		}
		a, b := []rune(before[2*i]), []rune(after[2*i])
		if string(a) == string(b) {
			t.Errorf("%s: the value was not replaced", name)
			continue
		}
		if !sameFormat(a, b) {
			t.Errorf("%s: the replacement changed the format", name)
		}
	}
}

// sameFormat reports whether b has the length of a, its other characters in the same places, and letters and
// digits, in the same case, where a has them.
func sameFormat(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		switch {
		case unicode.IsDigit(a[i]):
			if !unicode.IsDigit(b[i]) {
				return false
			}
		case unicode.IsLetter(a[i]):
			if !unicode.IsLetter(b[i]) || unicode.IsUpper(a[i]) != unicode.IsUpper(b[i]) {
				return false
			}
		case a[i] != b[i]:
			return false
		}
	}
	return true
}

// like returns a valid value of the detector shaped like template: its separators and case, and what
// synth.Like keeps (country, brand, class).
func like(t *testing.T, name, template string) string {
	t.Helper()
	d, _ := detectors.Get(name)
	g, ok := synth.Like(name, template)
	if !ok {
		t.Fatalf("no shaper for %s", name)
	}
	r := rand.New(rand.NewSource(1))
	for range 10000 {
		var src []rune
		for _, c := range g(r) {
			if unicode.IsLetter(c) || unicode.IsDigit(c) {
				src = append(src, c)
			}
		}
		out := []rune(template)
		k := 0
		for i, c := range out {
			if (unicode.IsLetter(c) || unicode.IsDigit(c)) && k < len(src) {
				out[i] = src[k]
				if unicode.IsLower(c) {
					out[i] = unicode.ToLower(out[i])
				}
				k++
			}
		}
		if v := string(out); k == len(src) && d.Pattern.FindString(v) == v && d.Validate(d.Normalize(v)) {
			return v
		}
	}
	t.Fatalf("no valid %s like the template", name)
	return ""
}

// TestFixKeepsCountryBrandAndFormat: a Romanian IBAN stays Romanian, a Visa stays a Visa, and each keeps how it
// was written.
func TestFixKeepsCountryBrandAndFormat(t *testing.T) {
	values := []struct{ name, keyword, value, keep string }{
		{"iban", "iban", like(t, "iban", "RO00 AAAA 0000 0000 0000 0000"), "RO"},
		{"iban", "iban", like(t, "iban", "nl00aaaa0000000000"), "nl"},
		{"card", "card", like(t, "card", "4000-0000-0000-0000"), "4"},
		{"card", "card", like(t, "card", "5100 0000 0000 0000"), "51"},
		{"card", "card", like(t, "card", "3700 000000 00000"), "37"},
		{"eu-phone", "phone", like(t, "eu-phone", "+40 721 000 000"), "+40 72"},
		{"email", "email", like(t, "email", "ana.pop12@firma.ro"), ""},
		{"ro-cnp", "cnp", like(t, "ro-cnp", "2000101400000"), "2"},
		{"eu-vat", "vat", like(t, "eu-vat", "NL000000000B01"), "NL"},
	}
	dir := t.TempDir()
	var body strings.Builder
	for _, v := range values {
		body.WriteString(v.keyword + ": " + v.value + "\n")
	}
	writeFile(t, dir, "tests/fixtures/accounts.txt", body.String())
	if code, out, errOut := run("fix", "--no-git", "--min-confidence", "low", dir); code != ExitClean || strings.Contains(out, "Nothing to fix") {
		t.Fatalf("fix: exit %d out %q err %q", code, out, errOut)
	}
	lines := strings.Split(readFile(t, dir, "tests/fixtures/accounts.txt"), "\n")
	for i, v := range values {
		got := strings.TrimPrefix(lines[i], v.keyword+": ")
		switch {
		case got == v.value:
			t.Errorf("%s %d: not replaced", v.name, i)
		case !sameFormat([]rune(v.value), []rune(got)):
			t.Errorf("%s %d: the format changed", v.name, i)
		case !strings.HasPrefix(got, v.keep):
			t.Errorf("%s %d: the country, brand or class changed", v.name, i)
		case v.name == "card" && detectors.CardBrand(detectors.DigitsOnly(got)) != detectors.CardBrand(detectors.DigitsOnly(v.value)):
			t.Errorf("%s %d: the brand changed", v.name, i)
		case v.name == "email" && !strings.HasSuffix(got, ".ro"):
			t.Errorf("%s %d: the top-level domain changed", v.name, i)
		}
	}
	if _, out, _ := run("fix", "--no-git", "--min-confidence", "low", dir); !strings.Contains(out, "Nothing to fix.") {
		t.Fatalf("a second fix should have nothing to do: %q", out)
	}
}

// TestFixReplacesKeywordValuesBelowTheReportLevel: a value next to its own keyword is real data in a fixture,
// even when a table of numbers and the test path lowered it below the level a scan reports.
func TestFixReplacesKeywordValuesBelowTheReportLevel(t *testing.T) {
	dir := t.TempDir()
	egn := generate(t, "bg-egn", 7)
	writeFile(t, dir, "tests/fixtures/people.json", `{"egn": "`+egn+`", "ref": "1234567890123", "n": "9876543210"}`+"\n")
	for _, f := range fixScanJSON(t, dir).Findings {
		if f.Detector == "bg-egn" {
			t.Fatalf("the EGN should be below the default report level for this test: %+v", f)
		}
	}
	if code, out, errOut := run("fix", "--no-git", dir); code != ExitClean || !strings.Contains(out, "replaced 1 value") {
		t.Fatalf("fix: exit %d out %q err %q", code, out, errOut)
	}
	if strings.Contains(readFile(t, dir, "tests/fixtures/people.json"), egn) {
		t.Fatal("the EGN next to its keyword was left in place")
	}
}
