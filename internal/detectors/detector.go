// Package detectors defines what a detector is and holds the registry. Each identifier lives in its own file.
package detectors

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Detector finds one kind of identifier. Pattern finds candidates; Validate confirms them with the
// identifier's own rules (check digits, dates, lengths). A detector without Validate reports every
// candidate at its base confidence.
type Detector struct {
	Name        string
	Label       string
	Country     string
	OnByDefault bool
	Pattern     *regexp.Regexp
	Normalize   func(string) string
	Validate    func(string) bool
	Mask        func(string) string
	Keywords    []string
	Base        domain.Confidence
	Samples     map[string]bool
	Validation  string
}

var registry = map[string]Detector{}

// Register adds a detector. It panics on a duplicate name, which is a programming error.
func Register(d Detector) {
	if _, dup := registry[d.Name]; dup {
		panic("detectors: duplicate name " + d.Name)
	}
	if d.Normalize == nil {
		d.Normalize = func(s string) string { return s }
	}
	if d.Mask == nil {
		d.Mask = MaskAll
	}
	registry[d.Name] = d
}

// All returns every registered detector, sorted by name.
func All() []Detector {
	out := make([]Detector, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns a detector by name.
func Get(name string) (Detector, bool) {
	d, ok := registry[name]
	return d, ok
}

// DigitsOnly removes everything that is not 0-9.
func DigitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Digits converts a string of ASCII digits to ints; ok is false for any other character.
func Digits(s string) ([]int, bool) {
	out := make([]int, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return nil, false
		}
		out[i] = int(c - '0')
	}
	return out, true
}

// MaskAll hides the whole value and keeps only its length.
func MaskAll(v string) string {
	return strings.Repeat("•", len([]rune(v)))
}

// MaskKeepLast hides all but the last n characters.
func MaskKeepLast(n int) func(string) string {
	return func(v string) string {
		r := []rune(v)
		if len(r) <= n {
			return MaskAll(v)
		}
		return strings.Repeat("•", len(r)-n) + string(r[len(r)-n:])
	}
}

// ValidDate checks a calendar date, including leap years.
func ValidDate(year, month, day int) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}
	days := [13]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if month == 2 && (year%4 == 0 && (year%100 != 0 || year%400 == 0)) {
		return day <= 29
	}
	return day <= days[month]
}
