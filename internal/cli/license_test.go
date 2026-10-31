package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/license"
)

// withLicense points the license seams at a test key, an empty config folder, the given environment and a fixed
// day, and returns a function that signs terms with the test key.
func withLicense(t *testing.T, env map[string]string, today string) (sign func(expires string) string, config string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config = t.TempDir()
	day, _ := time.Parse(time.DateOnly, today)
	oldEnv, oldDir, oldNow, oldKeys := getenv, configDir, now, trustedKeys
	t.Cleanup(func() { getenv, configDir, now, trustedKeys = oldEnv, oldDir, oldNow, oldKeys })
	getenv = func(k string) string { return env[k] }
	configDir = func() (string, error) { return config, nil }
	now = func() time.Time { return day.Add(12 * time.Hour) }
	trustedKeys = func() (license.Keys, error) { return license.Keys{"test": pub}, nil }
	return func(expires string) string {
		token, err := license.Sign(license.License{ID: "lic-1", Org: "Example Org", Plan: "privacy", Seats: 5,
			Issued: "2025-01-01", Expires: expires, Key: "test"}, priv)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}, config
}

func TestLicenseStatusWithoutALicense(t *testing.T) {
	withLicense(t, nil, "2026-11-01")
	code, out, _ := run("license")
	if code != ExitFindings || !strings.Contains(out, "No license found") {
		t.Fatalf("exit %d: %s", code, out)
	}
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "nothing personal\n")
	if code, _, errOut := run("scan", "--no-git", dir); code != ExitClean || !strings.Contains(errOut, "unlicensed (evaluation)") {
		t.Fatalf("an unlicensed scan still runs and says so: exit %d, %q", code, errOut)
	}
}

func TestLicenseInstallAndStatus(t *testing.T) {
	sign, config := withLicense(t, nil, "2026-11-01")
	file := filepath.Join(t.TempDir(), "acme.license")
	writeFile(t, filepath.Dir(file), "acme.license", "# for Example Org\n"+sign("2027-10-31")+"\n")
	code, out, errOut := run("license", "install", file)
	if code != ExitClean || !strings.Contains(out, "Example Org") {
		t.Fatalf("install: exit %d %q %q", code, out, errOut)
	}
	installed := filepath.Join(config, "ato", "license")
	if st, err := os.Stat(installed); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("installed file: %v %v", st, err)
	}
	code, out, _ = run("license", "status")
	if code != ExitClean || !strings.Contains(out, "valid, 364 days left") || !strings.Contains(out, installed) {
		t.Fatalf("status: exit %d %s", code, out)
	}
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "nothing personal\n")
	if _, _, errOut := run("scan", "--no-git", dir); errOut != "" {
		t.Fatalf("a licensed scan prints nothing about the license: %q", errOut)
	}
}

func TestLicenseInstallRefusesABadFile(t *testing.T) {
	sign, config := withLicense(t, nil, "2026-11-01")
	dir := t.TempDir()
	token := sign("2027-10-31")
	writeFile(t, dir, "edited.license", strings.Replace(token, token[len(token)-6:], "AAAAAA", 1))
	writeFile(t, dir, "old.license", sign("2025-06-30"))
	for _, name := range []string{"edited.license", "old.license", "missing.license"} {
		if code, _, _ := run("license", "install", filepath.Join(dir, name)); code != ExitUsage {
			t.Errorf("%s: exit %d, want %d", name, code, ExitUsage)
		}
	}
	if _, err := os.Stat(filepath.Join(config, "ato", "license")); err == nil {
		t.Fatal("nothing should be installed")
	}
}

func TestLicenseFromTheEnvironment(t *testing.T) {
	env := map[string]string{}
	sign, _ := withLicense(t, env, "2027-11-10")
	env[licenseEnv] = sign("2027-10-31")
	code, out, _ := run("license")
	if code != ExitClean || !strings.Contains(out, "grace period, 20 days left") || !strings.Contains(out, licenseEnv) {
		t.Fatalf("token in the environment: exit %d %s", code, out)
	}
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "nothing personal\n")
	if _, _, errOut := run("scan", "--no-git", dir); !strings.Contains(errOut, "expired on 2027-10-31; it keeps working for 20 more days") {
		t.Fatalf("grace notice: %q", errOut)
	}
	file := filepath.Join(t.TempDir(), "ci.license")
	writeFile(t, filepath.Dir(file), "ci.license", sign("2028-01-31"))
	env[licenseEnv] = file
	if code, out, _ := run("license"); code != ExitClean || !strings.Contains(out, "valid") || !strings.Contains(out, file) {
		t.Fatalf("path in the environment: exit %d %s", code, out)
	}
	env[licenseEnv] = sign("2027-01-31")
	if code, out, _ := run("license"); code != ExitFindings || !strings.Contains(out, "expired; ATO runs unlicensed") {
		t.Fatalf("expired past the grace period: exit %d %s", code, out)
	}
}
