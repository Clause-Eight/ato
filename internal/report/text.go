package report

import (
	"fmt"
	"io"
	"strings"
)

func init() { register("text", writeText) }

func writeText(w io.Writer, r Report, opts Options) error {
	var b strings.Builder
	current := ""
	files := 0
	for _, f := range r.Findings {
		if f.Location.Path != current {
			if current != "" {
				b.WriteString("\n")
			}
			current = f.Location.Path
			files++
			b.WriteString(current + "\n")
		}
		value := f.Masked
		if opts.ShowValues {
			value = f.Value
		}
		fmt.Fprintf(&b, "  %d:%d  %-6s  %s  %s\n", f.Location.Line, f.Location.Column, f.Confidence, f.Label, value)
		if len(f.Reasons) > 0 {
			fmt.Fprintf(&b, "          %s\n", strings.Join(f.Reasons, "; "))
		}
	}
	if len(r.Findings) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(summaryLine(r, files) + "\n")
	for _, s := range r.Summary.Unreadable {
		fmt.Fprintf(&b, "pii-eu: could not read %s (%s)\n", s.Path, s.Reason)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func summaryLine(r Report, files int) string {
	n := len(r.Findings)
	parts := []string{}
	switch n {
	case 0:
		parts = append(parts, "No personal data found")
	case 1:
		parts = append(parts, "1 finding in 1 file")
	default:
		parts = append(parts, fmt.Sprintf("%d findings in %d %s", n, files, plural(files, "file", "files")))
	}
	parts = append(parts, fmt.Sprintf("%d %s scanned", r.Summary.FilesScanned, plural(r.Summary.FilesScanned, "file", "files")))
	if k := len(r.Summary.FilesSkipped); k > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", k))
	}
	if k := len(r.Summary.Unreadable); k > 0 {
		parts = append(parts, fmt.Sprintf("%d unreadable", k))
	}
	return strings.Join(parts, " · ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
