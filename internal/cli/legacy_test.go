package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests in this file cover the names used before the rename to ATO, which 2.x still reads with a warning
// (docs/COMPATIBILITY.md, "Changes in 2.0").

func TestLegacySettingsFileIsReadWithAWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pay.txt", "card: "+syntheticCard()+"\n")
	writeFile(t, dir, ".pii-eu.toml", "[detectors]\ncard = false\n")
	code, _, errOut := run("scan", "--no-git", dir)
	if code != ExitClean {
		t.Fatalf("the legacy settings file was not read: exit %d (%s)", code, errOut)
	}
	if !strings.Contains(errOut, ".pii-eu.toml is deprecated") || !strings.Contains(errOut, ".ato.toml") {
		t.Errorf("no deprecation warning: %q", errOut)
	}

	writeFile(t, dir, ".ato.toml", "[detectors]\ncard = true\n")
	code, _, errOut = run("scan", "--no-git", dir)
	if code != ExitFindings {
		t.Fatalf(".ato.toml should win over .pii-eu.toml: exit %d (%s)", code, errOut)
	}
	if strings.Contains(errOut, "deprecated") {
		t.Errorf("warning although .ato.toml exists: %q", errOut)
	}
}

func TestLegacyBaselineIsReadWithAWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pay.txt", "card: "+syntheticCard()+"\n")
	if code, _, errOut := run("baseline", "--no-git", dir); code != ExitClean {
		t.Fatalf("baseline: %d %s", code, errOut)
	}
	if err := os.Rename(filepath.Join(dir, ".ato-baseline.json"), filepath.Join(dir, ".pii-eu-baseline.json")); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run("scan", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "1 in the baseline") {
		t.Fatalf("the legacy baseline was not read: %d %q %q", code, out, errOut)
	}
	if !strings.Contains(errOut, ".pii-eu-baseline.json is deprecated") {
		t.Errorf("no deprecation warning: %q", errOut)
	}

	code, _, errOut = run("baseline", "--no-git", dir)
	if code != ExitClean || !strings.Contains(errOut, "delete the deprecated") {
		t.Fatalf("baseline next to a legacy file: %d %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ato-baseline.json")); err != nil {
		t.Fatal("baseline did not write .ato-baseline.json")
	}
}

func TestLegacyIgnoreCommentIsHonouredWithAWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pay.txt", "card: "+syntheticCard()+" // pii-eu:ignore synthetic test card\n")
	code, out, errOut := run("scan", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "ignored") {
		t.Fatalf("the legacy ignore comment was not honoured: %d %q %q", code, out, errOut)
	}
	if !strings.Contains(errOut, "deprecated pii-eu:ignore") || !strings.Contains(errOut, "pay.txt") {
		t.Errorf("no deprecation warning: %q", errOut)
	}

	writeFile(t, dir, "pay.txt", "card: "+syntheticCard()+" // ato:ignore synthetic test card\n")
	code, _, errOut = run("scan", "--no-git", dir)
	if code != ExitClean || strings.Contains(errOut, "deprecated") {
		t.Fatalf("ato:ignore: %d %q", code, errOut)
	}
}
