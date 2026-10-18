// Package config loads `.pii-eu.toml` and merges command-line overrides.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// FileName is the configuration file looked up in the scanned folder.
const FileName = ".pii-eu.toml"

// DefaultMaxFileSize skips files larger than 5 MiB.
const DefaultMaxFileSize = 5 << 20

// Config is the effective configuration for one scan.
type Config struct {
	Include       []string
	Exclude       []string
	MinConfidence domain.Confidence
	MaxFileSize   int64
	Detectors     map[string]bool
	// DetectorMinConfidence overrides MinConfidence for single detectors.
	DetectorMinConfidence map[string]domain.Confidence
	Countries             domain.Countries
}

type fileConfig struct {
	Include       []string        `toml:"include"`
	Exclude       []string        `toml:"exclude"`
	MinConfidence string          `toml:"min_confidence"`
	MaxFileSize   int64           `toml:"max_file_size"`
	Detectors     map[string]bool `toml:"detectors"`
	Countries     []string        `toml:"countries"`
	// DetectorMinConfidence is the [min_confidence_per_detector] table.
	DetectorMinConfidence map[string]string `toml:"min_confidence_per_detector"`
}

// Default is the configuration used when no file and no flags change it.
func Default() Config {
	return Config{MinConfidence: domain.Medium, MaxFileSize: DefaultMaxFileSize, Detectors: map[string]bool{}, DetectorMinConfidence: map[string]domain.Confidence{}}
}

// Load reads path, or returns Default when explicit is false and the file does not exist.
func Load(path string, explicit bool) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	var fc fileConfig
	md, err := toml.Decode(string(data), &fc)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return cfg, fmt.Errorf("%s: unknown setting %q", filepath.Base(path), undecoded[0].String())
	}
	return apply(cfg, fc)
}

func apply(cfg Config, fc fileConfig) (Config, error) {
	cfg.Include = fc.Include
	cfg.Exclude = fc.Exclude
	if fc.MinConfidence != "" {
		c, err := domain.ParseConfidence(fc.MinConfidence)
		if err != nil {
			return cfg, fmt.Errorf("min_confidence: %w", err)
		}
		cfg.MinConfidence = c
	}
	if fc.MaxFileSize < 0 {
		return cfg, errors.New("max_file_size must be zero or more")
	}
	if fc.MaxFileSize > 0 {
		cfg.MaxFileSize = fc.MaxFileSize
	}
	for name, on := range fc.Detectors {
		cfg.Detectors[name] = on
	}
	for name, level := range fc.DetectorMinConfidence {
		c, err := domain.ParseConfidence(level)
		if err != nil {
			return cfg, fmt.Errorf("min_confidence_per_detector.%s: %w", name, err)
		}
		cfg.DetectorMinConfidence[name] = c
	}
	countries, err := domain.ParseCountries(fc.Countries)
	if err != nil {
		return cfg, fmt.Errorf("countries: %w", err)
	}
	cfg.Countries = countries
	return cfg, nil
}

// MinConfidenceFor is the lowest confidence reported for a detector.
func (c Config) MinConfidenceFor(detector string) domain.Confidence {
	if min, ok := c.DetectorMinConfidence[detector]; ok {
		return min
	}
	return c.MinConfidence
}

// Enabled reports whether a detector should run. A detector switched on or off by name follows that setting.
// Otherwise a national detector runs when its country is selected by name or preset, even if it is off by
// default (the EEA, Swiss and UK pack), and every other detector runs when it is on by default and its country
// is included.
func (c Config) Enabled(name string, countries []string, onByDefault bool) bool {
	if on, ok := c.Detectors[name]; ok {
		return on
	}
	for _, country := range countries {
		if !c.Countries.All() && country != domain.CrossBorder && c.Countries[country] {
			return true
		}
		if onByDefault && c.Countries.Includes(country) {
			return true
		}
	}
	return false
}
