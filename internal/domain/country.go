package domain

import (
	"fmt"
	"sort"
	"strings"
)

// CrossBorder is the country of detectors that apply everywhere (IBAN, cards, contact data). They run whatever
// countries are selected.
const CrossBorder = "EU"

// MemberStates are the 27 EU member states, by ISO 3166-1 alpha-2 code. Greece is GR here, not EL as in VAT numbers.
var MemberStates = []string{"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "ES", "FI", "FR", "GR", "HR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK"}

// OtherCountries are covered outside the EU: the rest of the EEA, Switzerland and the United Kingdom.
var OtherCountries = []string{"CH", "GB", "IS", "LI", "NO"}

// Presets are named groups of countries for --countries.
var Presets = map[string][]string{
	"eu":      MemberStates,
	"eea":     append(append([]string{}, MemberStates...), "IS", "LI", "NO"),
	"cee":     {"BG", "CZ", "EE", "HR", "HU", "LT", "LV", "PL", "RO", "SI", "SK"},
	"dach":    {"AT", "CH", "DE"},
	"benelux": {"BE", "LU", "NL"},
	"nordics": {"DK", "FI", "IS", "NO", "SE"},
	"baltics": {"EE", "LT", "LV"},
}

// Countries is a selection of countries. The zero value selects every country.
type Countries map[string]bool

// All reports whether no selection was made, so every detector runs.
func (c Countries) All() bool { return len(c) == 0 }

// Includes reports whether detectors for country should run. Cross-border detectors always run.
func (c Countries) Includes(country string) bool {
	return c.All() || country == CrossBorder || c[country]
}

// ParseCountries reads country codes and preset names, case-insensitively. An unknown code is an error.
func ParseCountries(items []string) (Countries, error) {
	known := map[string]bool{}
	for _, cc := range append(append([]string{}, MemberStates...), OtherCountries...) {
		known[cc] = true
	}
	out := Countries{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if preset, ok := Presets[strings.ToLower(item)]; ok {
			for _, cc := range preset {
				out[cc] = true
			}
			continue
		}
		cc := strings.ToUpper(item)
		if cc == "EL" {
			cc = "GR"
		}
		if cc == "UK" {
			cc = "GB"
		}
		if !known[cc] {
			return nil, fmt.Errorf("unknown country %q: use an ISO code such as RO or DE, or a preset (%s)", item, strings.Join(PresetNames(), ", "))
		}
		out[cc] = true
	}
	return out, nil
}

// PresetNames lists the presets in alphabetical order.
func PresetNames() []string {
	names := make([]string, 0, len(Presets))
	for n := range Presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
