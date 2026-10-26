// Package domain holds the vocabulary shared by every other package. It imports nothing from this module.
package domain

import "fmt"

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

// Finding is one detected value, a schema field holding special-category data, or a call that writes personal
// data to a log or telemetry sink (Kind says which). Value is the raw text and must never reach a report;
// reporters use Masked. For a field, Masked is the field's name; for a sink, the expression passed to the call.
// Span is where a value sits in its line, in bytes, so it can be replaced.
type Finding struct {
	Kind        Kind
	Category    DataCategory
	Special     SpecialCategory
	Sink        Sink
	Detector    string
	Label       string
	Country     string
	Location    Location
	Span        Span
	Value       string
	Masked      string
	Confidence  Confidence
	Reasons     []string
	Sample      bool
	TestPath    bool
	Fingerprint string
	Ignored     bool
}

// Span is a byte range within one line: Start is inclusive, End exclusive.
type Span struct {
	Start, End int
}

// Summary describes one scan.
type Summary struct {
	FilesScanned int
	FilesSkipped []Skipped
	Unreadable   []Skipped
	Ignored      int
	Baselined    int
}

// Skipped is a file that was not scanned, with the reason.
type Skipped struct {
	Path   string
	Reason string
}
