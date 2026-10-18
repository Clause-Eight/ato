package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
