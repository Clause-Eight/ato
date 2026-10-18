package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileGivesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), FileName), false)
	if err != nil || cfg.MinConfidence != domain.Medium || cfg.MaxFileSize != DefaultMaxFileSize {
		t.Fatalf("got %+v %v", cfg, err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "x.toml"), true); err == nil {
		t.Fatal("an explicit missing file must fail")
	}
}

func TestLoadSettings(t *testing.T) {
	p := write(t, `
include = ["src/**"]
exclude = ["vendor"]
min_confidence = "high"
max_file_size = 1000
[detectors]
email = true
ro-cnp = false
`)
	cfg, err := Load(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinConfidence != domain.High || cfg.MaxFileSize != 1000 || len(cfg.Include) != 1 || len(cfg.Exclude) != 1 {
		t.Fatalf("got %+v", cfg)
	}
	if !cfg.Enabled("email", false) || cfg.Enabled("ro-cnp", true) || !cfg.Enabled("iban", true) {
		t.Fatal("detector switches wrong")
	}
}

func TestBadSettingsAreErrors(t *testing.T) {
	for body, want := range map[string]string{
		`min_confidence = "very"`: "min_confidence",
		`max_file_size = -1`:      "max_file_size",
		`colour = "blue"`:         "unknown setting",
		`include = [`:             FileName,
	} {
		_, err := Load(write(t, body), true)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want error containing %q", body, err, want)
		}
	}
}
