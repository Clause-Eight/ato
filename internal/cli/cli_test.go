package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
)

func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"nope"}, {"scan", "a", "b"}, {"scan", "--format", "xml", "."}, {"scan", "--min-confidence", "very", "."}, {"scan", "--include", "", "."}} {
		code, _, _ := run(args...)
		if code != ExitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, ExitUsage)
		}
	}
}

func TestVersionAndHelp(t *testing.T) {
	if code, out, _ := run("version"); code != 0 || !strings.HasPrefix(out, "pii-eu ") {
		t.Fatalf("version: %d %q", code, out)
	}
	if code, out, _ := run("help"); code != 0 || !strings.Contains(out, "Exit codes") {
		t.Fatalf("help: %d", code)
	}
}

func TestCleanFolderExitsZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("nothing personal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run("scan", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "No personal data found") {
		t.Fatalf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestUnknownDetectorInSettings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("[detectors]\nnot-a-detector = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run("scan", dir)
	if code != ExitUsage || !strings.Contains(errOut, "unknown detector") {
		t.Fatalf("exit %d err %q", code, errOut)
	}
}

func TestFlagsAfterPath(t *testing.T) {
	got := reorder([]string{".", "--format", "json", "--no-git", "--include=x"})
	want := "--format json --no-git --include=x ."
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v", got)
	}
}

func syntheticCard() string {
	body := "453201511283036"
	for c := '0'; c <= '9'; c++ {
		if v := body + string(c); detectors.Luhn(v) {
			return v
		}
	}
	return ""
}

func TestScanWritesEachFormatWithoutValues(t *testing.T) {
	dir := t.TempDir()
	card := syntheticCard()
	if err := os.WriteFile(filepath.Join(dir, "pay.txt"), []byte("card: "+card+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"text", "json", "sarif"} {
		out := filepath.Join(t.TempDir(), "report."+format)
		code, _, errOut := run("scan", "--no-git", "--format", format, "--output", out, "--show-values", dir)
		if code != ExitFindings {
			t.Fatalf("%s: exit %d (%s)", format, code, errOut)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), card) {
			t.Errorf("%s report file contains the raw value", format)
		}
	}
}

func TestBaselineReportsOnlyNewFindings(t *testing.T) {
	dir := t.TempDir()
	card := syntheticCard()
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(dir, "pay.txt"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("card: " + card + "\n")
	if code, out, errOut := run("baseline", "--no-git", dir); code != ExitClean || !strings.Contains(out, "1 finding") {
		t.Fatalf("baseline: %d %q %q", code, out, errOut)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".pii-eu-baseline.json"))
	if strings.Contains(string(data), card) {
		t.Fatal("baseline contains the value")
	}
	if code, out, _ := run("scan", "--no-git", dir); code != ExitClean || !strings.Contains(out, "1 in the baseline") {
		t.Fatalf("baselined scan: %d %q", code, out)
	}
	write("card: " + card + "\nother card: " + card + "\n")
	if code, _, _ := run("scan", "--no-git", dir); code != ExitFindings {
		t.Fatalf("a new finding should fail: %d", code)
	}
}

func TestScanStdin(t *testing.T) {
	Stdin = strings.NewReader("log line card=" + syntheticCard() + "\n")
	defer func() { Stdin = os.Stdin }()
	code, out, _ := run("scan", "-")
	if code != ExitFindings || !strings.Contains(out, "<stdin>") {
		t.Fatalf("stdin: %d %q", code, out)
	}
}

func TestStagedScansTheStagedVersion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	p := filepath.Join(dir, "pay.txt")
	_ = os.WriteFile(p, []byte("card: "+syntheticCard()+"\n"), 0o600)
	git("add", "pay.txt")
	_ = os.WriteFile(p, []byte("clean now\n"), 0o600)
	if code, _, errOut := run("scan", "--staged", dir); code != ExitFindings {
		t.Fatalf("staged content should be scanned: %d %s", code, errOut)
	}
	if code, _, _ := run("scan", "--staged", t.TempDir()); code != ExitUsage {
		t.Fatalf("--staged outside git should be a usage error, got %d", code)
	}
}

func TestDetectorReferenceIsUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "DETECTORS.md")
	if os.Getenv("PII_EU_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(DetectorTable()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != DetectorTable() {
		t.Fatal("docs/DETECTORS.md is out of date: run PII_EU_UPDATE_GOLDEN=1 go test ./internal/cli")
	}
}
