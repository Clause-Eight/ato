// Package report writes findings in each output format. Reporters only ever see masked values.
package report

import (
	"fmt"
	"io"
	"sort"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Product is the product name printed in reports for people and in the SARIF driver. Report.Tool is the
// command name written in machine-readable reports.
const Product = "ATO"

// Report is everything a reporter may print. Findings carry no raw values: Redact removes them first.
type Report struct {
	Tool     string
	Version  string
	Root     string
	Findings []domain.Finding
	Summary  domain.Summary
}

// Options change how a report is written.
type Options struct {
	ShowValues bool
}

// Writer writes one format.
type Writer func(w io.Writer, r Report, opts Options) error

var writers = map[string]Writer{}

func register(name string, w Writer) { writers[name] = w }

// Formats lists the available formats.
func Formats() []string {
	out := make([]string, 0, len(writers))
	for k := range writers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Write writes r in the named format.
func Write(format string, w io.Writer, r Report, opts Options) error {
	fn, ok := writers[format]
	if !ok {
		return fmt.Errorf("unknown format %q: use one of %v", format, Formats())
	}
	if !opts.ShowValues || format != "text" {
		r.Findings = Redact(r.Findings)
		opts.ShowValues = false
	}
	return fn(w, r, opts)
}

// Redact returns copies of the findings with the raw value removed.
func Redact(fs []domain.Finding) []domain.Finding {
	out := make([]domain.Finding, len(fs))
	for i, f := range fs {
		f.Value = ""
		out[i] = f
	}
	return out
}
