package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeygenIssueInspect(t *testing.T) {
	dir := t.TempDir()
	today := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	var out bytes.Buffer
	if err := run([]string{"keygen", "-id", "test-2026", "-out", dir}, &out, today); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "test-2026.key")
	if st, err := os.Stat(key); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("private key file: %v %v", st, err)
	}
	if err := run([]string{"keygen", "-id", "test-2026", "-out", dir}, &out, today); err == nil {
		t.Fatal("keygen must not overwrite a key")
	}
	lic, ledger := filepath.Join(dir, "example.license"), filepath.Join(dir, "issued.csv")
	err := run([]string{"issue", "-key", key, "-org", "Example Org, Ltd", "-plan", "privacy", "-seats", "10",
		"-features", "map, fix", "-expires", "2027-10-31", "-out", lic, "-ledger", ledger}, &out, today)
	if err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"issue", "-key", key, "-org", "Example Org", "-plan", "privacy", "-expires", "2027-10-31", "-out", lic}, &out, today); err == nil {
		t.Fatal("issue must not overwrite a license file")
	}
	out.Reset()
	if err := run([]string{"inspect", lic, "-pub", filepath.Join(dir, "test-2026.pub")}, &out, today); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "valid") || !strings.Contains(got, "org Example Org, Ltd") || !strings.Contains(got, "features map,fix") || !strings.Contains(got, "2026-11-01 to 2027-10-31, key test-2026") {
		t.Fatalf("inspect: %s", got)
	}
	if err := run([]string{"inspect", lic}, &out, today); err == nil {
		t.Fatal("a test key is not built into ATO, so inspecting with the built-in keys must fail")
	}
	rows, _ := os.ReadFile(ledger)
	if lines := strings.Split(strings.TrimSpace(string(rows)), "\n"); len(lines) != 2 || !strings.Contains(lines[1], `"Example Org, Ltd",privacy,10,map fix,2026-11-01,2027-10-31,test-2026`) {
		t.Fatalf("ledger: %q", rows)
	}
	if data, _ := os.ReadFile(lic); bytes.Contains(data, []byte(strings.TrimSpace(readFile(t, key)))) {
		t.Fatal("the license must not contain the private key")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
