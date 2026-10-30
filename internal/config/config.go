// Package config loads `.ato.toml` and merges command-line overrides.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Clause-Eight/ato/internal/domain"
)

// FileName is the configuration file looked up in the scanned folder.
const FileName = ".ato.toml"

// LegacyFileName is the settings file name before the rename to ATO. It is read, with a warning, when FileName
// is absent; deprecated in 2.0.
const LegacyFileName = ".pii-eu.toml"

// DefaultMaxFileSize skips files larger than 5 MiB.
const DefaultMaxFileSize = 5 << 20

// DefaultMaxRows is how many rows of a CSV, TSV or JSON Lines file are read.
const DefaultMaxRows = 100_000

// DefaultImageExclude skips the folders of operating system packages inside container images.
var DefaultImageExclude = []string{"/usr/share/", "/usr/lib/", "/usr/lib64/", "/usr/libexec/", "/usr/include/", "/lib/", "/lib64/", "/var/lib/dpkg/", "/var/lib/apt/", "/var/lib/rpm/", "/var/cache/", "/etc/ssl/certs/", "/proc/", "/sys/"}

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
	// MaxRows caps the rows read from each data file; 0 reads every row.
	MaxRows int
	// Archive limits; 0 takes the built-in default.
	MaxArchiveBytes   int64
	MaxArchiveEntries int64
	MaxArchiveDepth   int
	// ImageExclude is excluded inside container images, in addition to Exclude.
	ImageExclude []string
	// AllowPartial keeps the exit code at 0 or 1 when files were not read in full (ADR 8).
	AllowPartial bool
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
	MaxRows               *int              `toml:"max_rows"`
	MaxArchiveBytes       int64             `toml:"max_archive_bytes"`
	MaxArchiveEntries     int64             `toml:"max_archive_entries"`
	MaxArchiveDepth       int               `toml:"max_archive_depth"`
	ImageExclude          *[]string         `toml:"image_exclude"`
	AllowPartial          bool              `toml:"allow_partial"`
}

// Default is the configuration used when no file and no flags change it.
func Default() Config {
	return Config{MinConfidence: domain.Medium, MaxFileSize: DefaultMaxFileSize, Detectors: map[string]bool{}, DetectorMinConfidence: map[string]domain.Confidence{},
		MaxRows: DefaultMaxRows, ImageExclude: append([]string{}, DefaultImageExclude...)}
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
	cfg.AllowPartial = fc.AllowPartial
	if fc.MaxRows != nil {
		if *fc.MaxRows < 0 {
			return cfg, errors.New("max_rows must be zero (every row) or more")
		}
		cfg.MaxRows = *fc.MaxRows
	}
	if fc.MaxArchiveBytes < 0 || fc.MaxArchiveEntries < 0 || fc.MaxArchiveDepth < 0 {
		return cfg, errors.New("max_archive_bytes, max_archive_entries and max_archive_depth must be zero (the default) or more")
	}
	cfg.MaxArchiveBytes, cfg.MaxArchiveEntries, cfg.MaxArchiveDepth = fc.MaxArchiveBytes, fc.MaxArchiveEntries, fc.MaxArchiveDepth
	if fc.ImageExclude != nil {
		cfg.ImageExclude = *fc.ImageExclude
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
