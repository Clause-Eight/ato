// Package baseline stores fingerprints of accepted findings so that only new ones are reported.
// Fingerprints are derived from the detector, path and the line with values replaced, never from a value.
package baseline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/Clause-Eight/ato/internal/domain"
)

// FileName is the default baseline file in the scanned folder.
const FileName = ".pii-eu-baseline.json"

const version = 1

// Baseline is a set of fingerprints.
type Baseline map[string]bool

type file struct {
	Version      int      `json:"version"`
	Note         string   `json:"note"`
	Fingerprints []string `json:"fingerprints"`
}

// Load reads a baseline. A missing file is an empty baseline unless explicit is true.
func Load(path string, explicit bool) (Baseline, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		return Baseline{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s is not a valid baseline: %w", path, err)
	}
	if f.Version != version {
		return nil, fmt.Errorf("%s has baseline version %d; this pii-eu reads version %d", path, f.Version, version)
	}
	b := Baseline{}
	for _, fp := range f.Fingerprints {
		b[fp] = true
	}
	return b, nil
}

// Save writes the fingerprints of findings, sorted, so the file diffs cleanly.
func Save(path string, findings []domain.Finding) error {
	seen := map[string]bool{}
	fps := []string{}
	for _, f := range findings {
		if !seen[f.Fingerprint] {
			seen[f.Fingerprint] = true
			fps = append(fps, f.Fingerprint)
		}
	}
	sort.Strings(fps)
	data, err := json.MarshalIndent(file{
		Version:      version,
		Note:         "Findings accepted when this baseline was made. Fingerprints contain no personal data. Regenerate with `pii-eu baseline`.",
		Fingerprints: fps,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Filter returns the findings not in the baseline and how many were left out.
func (b Baseline) Filter(findings []domain.Finding) ([]domain.Finding, int) {
	var out []domain.Finding
	dropped := 0
	for _, f := range findings {
		if b[f.Fingerprint] {
			dropped++
			continue
		}
		out = append(out, f)
	}
	return out, dropped
}
