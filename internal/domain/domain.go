// Package domain holds the vocabulary shared by every other package. It imports nothing from this module.
package domain

import (
	"fmt"
	"slices"
	"strings"
)

// Confidence says how sure a finding is. Higher is surer.
type Confidence int

const (
	Low Confidence = iota + 1
	Medium
	High
)

func (c Confidence) String() string {
	switch c {
	case Low:
		return "low"
	case Medium:
		return "medium"
	case High:
		return "high"
	}
	return "unknown"
}

// ParseConfidence reads "low", "medium" or "high".
func ParseConfidence(s string) (Confidence, error) {
	switch s {
	case "low":
		return Low, nil
	case "medium":
		return Medium, nil
	case "high":
		return High, nil
	}
	return 0, fmt.Errorf("unknown confidence %q: use low, medium or high", s)
}

// Location is where a value was found. Line and Column start at 1.
type Location struct {
	Path   string
	Line   int
	Column int
}

// Finding is one detected value, a schema field holding special-category data, a call that writes personal data
// to a log or telemetry sink, a data store declared in infrastructure code, or a file that holds personal data as
// a whole (Kind says which). Value is the raw text and must never reach a report; reporters use Masked. For a
// field, Masked is the field's name; for a sink, the expression passed to the call; for a store, its resource
// address; for a file, what it holds with any value masked.
// ColumnName is the data column a value was found in (a CSV or TSV header, a JSON Lines key path or a SQL
// column), when the file has columns. Span is where a value sits in its line, in bytes, so it can be replaced.
type Finding struct {
	Kind       Kind
	Category   DataCategory
	Special    SpecialCategory
	Sink       Sink
	Detector   string
	Label      string
	Country    string
	Location   Location
	ColumnName string
	Span       Span
	Value      string
	Masked     string
	Confidence Confidence
	Reasons    []string
	Sample     bool
	TestPath   bool
	// Keyword is true when the identifier's own name or keyword was found next to the value.
	Keyword     bool
	Fingerprint string
	Ignored     bool
	// LegacyIgnore is true when the ignore comment used the deprecated pii-eu:ignore form.
	LegacyIgnore bool
	// Store is the data store an infra finding is about; nil for other kinds.
	Store *Store
	// Possible lists the other identifiers a value is just as valid for, when nothing near it says which it is.
	Possible []Possible
}

// Possible is another identifier a value could equally be.
type Possible struct {
	Detector, Label, Country string
}

// Title is what a report calls a finding: the detector's label, or "Possibly A or B" when the value is just as
// valid for other identifiers.
func (f Finding) Title() string {
	if len(f.Possible) == 0 {
		return f.Label
	}
	labels := []string{f.Label}
	for _, p := range f.Possible {
		labels = append(labels, p.Label)
	}
	last := len(labels) - 1
	return "Possibly " + strings.Join(labels[:last], ", ") + " or " + labels[last]
}

// Countries is the finding's country followed by those of the other possible identifiers, without repeats.
func (f Finding) Countries() []string {
	out := []string{f.Country}
	for _, p := range f.Possible {
		if !slices.Contains(out, p.Country) {
			out = append(out, p.Country)
		}
	}
	return out
}

// Span is a byte range within one line: Start is inclusive, End exclusive.
type Span struct {
	Start, End int
}

// Summary describes one scan. Sampled lists data files of which only the first rows were read.
type Summary struct {
	FilesScanned int
	FilesSkipped []Skipped
	Unreadable   []Skipped
	Sampled      []Skipped
	Ignored      int
	Baselined    int
}

// Skipped is a file that was not scanned, with the reason.
type Skipped struct {
	Path   string
	Reason string
	// NotRead marks a file that may hold personal data and was not read in full: over a size limit, cut at
	// max_rows, nested too deep, or in a document format ATO does not open. It makes a scan incomplete.
	NotRead bool
}
