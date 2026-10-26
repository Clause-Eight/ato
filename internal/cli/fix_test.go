package cli

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/synth"
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
}
