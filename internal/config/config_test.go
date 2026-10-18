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
	if !cfg.Enabled("email", []string{"EU"}, false) || cfg.Enabled("ro-cnp", []string{"RO"}, true) || !cfg.Enabled("iban", []string{"EU"}, true) {
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

func TestCountriesAndPerDetectorConfidence(t *testing.T) {
	p := write(t, `
countries = ["ro", "dach"]
[min_confidence_per_detector]
email = "high"
`)
	cfg, err := Load(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled("ro-cnp", []string{"RO"}, true) || !cfg.Enabled("de-tax-id", []string{"DE"}, true) || cfg.Enabled("bg-egn", []string{"BG"}, true) {
		t.Error("country selection not applied")
	}
	if !cfg.Enabled("iban", []string{"EU"}, true) || cfg.Enabled("ip-address", []string{"EU"}, false) {
		t.Error("cross-border detectors follow their default")
	}
	if !cfg.Enabled("ch-ahv", []string{"CH"}, false) {
		t.Error("a selected country turns on its opt-in detectors")
	}
	if cfg.MinConfidenceFor("email") != domain.High || cfg.MinConfidenceFor("iban") != domain.Medium {
		t.Error("per-detector confidence not applied")
	}
	for _, bad := range []string{`countries = ["XX"]`, "[min_confidence_per_detector]\nemail = \"loud\""} {
		if _, err := Load(write(t, bad), true); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
