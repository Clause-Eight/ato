package fix

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// File is one file to fix. Replace are the findings to replace; All are every value finding in the file, so
// the diff can mask values that are left in place.
type File struct {
	Path    string
	Content []byte
	Replace []domain.Finding
	All     []domain.Finding
}

// Outcome is what fixing one file did. Content is the new content; Diff shows the changed lines with found
// values masked. Stale counts findings whose text was no longer in the file, and Unfixable those with no
// generator or no fitting value.
type Outcome struct {
	Path      string
	Content   []byte
	Changes   []Change
	Diff      string
	Stale     int
	Unfixable int
}

// Change is one replaced value. Masked is the found value, masked.
type Change struct {
	Line, Column int
	Detector     string
	Masked       string
	Replacement  Replacement
}

type key struct{ detector, raw string }

// Run fixes the files, computing each distinct replacement once and in parallel.
func Run(files []File) []Outcome {
	split := make([][]string, len(files))
	wanted := map[key]bool{}
	for i, f := range files {
		split[i] = strings.Split(string(f.Content), "\n")
		for _, fd := range f.Replace {
			if raw, ok := rawText(split[i], fd); ok {
				wanted[key{fd.Detector, raw}] = true
			}
		}
	}
	values := replaceAll(wanted)
	out := make([]Outcome, 0, len(files))
	for i, f := range files {
		out = append(out, apply(f, split[i], values))
	}
	return out
}

// rawText is the text of a finding in its line, if it still normalises to the value that was found.
func rawText(lines []string, f domain.Finding) (string, bool) {
	n := f.Location.Line
	if n < 1 || n > len(lines) {
		return "", false
	}
	line := strings.TrimSuffix(lines[n-1], "\r")
	s, e := f.Span.Start, f.Span.End
	if s < 0 || e > len(line) || s >= e {
		return "", false
	}
	d, ok := detectors.Get(f.Detector)
	if !ok || d.Normalize(line[s:e]) != f.Value {
		return "", false
	}
	return line[s:e], true
}

type result struct {
	r  Replacement
	ok bool
}

func replaceAll(wanted map[key]bool) map[key]result {
	keys := make(chan key)
	out := map[key]result{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := range keys {
				d, _ := detectors.Get(k.detector)
				r, ok := Replace(d, k.raw)
				mu.Lock()
				out[k] = result{r, ok}
				mu.Unlock()
			}
		}()
	}
	for k := range wanted {
		keys <- k
	}
	close(keys)
	wg.Wait()
	return out
}

type edit struct {
	span domain.Span
	text string
}

func apply(f File, lines []string, values map[key]result) Outcome {
	o := Outcome{Path: f.Path}
	byLine := map[int][]domain.Finding{}
	for _, fd := range f.Replace {
		byLine[fd.Location.Line] = append(byLine[fd.Location.Line], fd)
	}
	masks := map[int][]domain.Finding{}
	for _, fd := range f.All {
		if fd.Kind == domain.KindValue || fd.Kind == "" {
			masks[fd.Location.Line] = append(masks[fd.Location.Line], fd)
		}
	}
	numbers := make([]int, 0, len(byLine))
	for n := range byLine {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	var diff strings.Builder
	for _, n := range numbers {
		var edits []edit
		for _, fd := range byLine[n] {
			raw, ok := rawText(lines, fd)
			if !ok {
				o.Stale++
				continue
			}
			res := values[key{fd.Detector, raw}]
			if !res.ok {
				o.Unfixable++
				continue
			}
			edits = append(edits, edit{fd.Span, res.r.Text})
			o.Changes = append(o.Changes, Change{Line: n, Column: fd.Location.Column, Detector: fd.Detector, Masked: fd.Masked, Replacement: res.r})
		}
		if len(edits) == 0 {
			continue
		}
		line := lines[n-1]
		cr := strings.HasSuffix(line, "\r")
		line = strings.TrimSuffix(line, "\r")
		lines[n-1] = rewrite(line, edits, nil)
		if cr {
			lines[n-1] += "\r"
		}
		if diff.Len() == 0 {
			diff.WriteString("--- a/" + f.Path + "\n+++ b/" + f.Path + "\n")
		}
		old := rewrite(line, nil, masked(append(append([]domain.Finding{}, byLine[n]...), masks[n]...), nil))
		diff.WriteString(hunk(n, old, rewrite(line, edits, masked(masks[n], edits))))
	}
	o.Diff = diff.String()
	o.Content = []byte(strings.Join(lines, "\n"))
	if len(o.Changes) == 0 {
		o.Content = f.Content
	}
	return o
}

// masked are the masks of the found values on a line that no edit replaces, so the diff never shows them.
func masked(fs []domain.Finding, edits []edit) []edit {
	var out []edit
	for _, fd := range fs {
		replaced := false
		for _, e := range edits {
			if e.span == fd.Span {
				replaced = true
			}
		}
		if !replaced {
			out = append(out, edit{fd.Span, fd.Masked})
		}
	}
	return out
}

// rewrite applies edits to a line, and writes the masks of found values over the spans in hide that no edit
// covers. Spans that overlap an earlier one or fall outside the line are left alone.
func rewrite(line string, edits, hide []edit) string {
	all := append(append([]edit{}, edits...), hide...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].span.Start < all[j].span.Start })
	var b strings.Builder
	last := 0
	for _, e := range all {
		if e.span.Start < last || e.span.End > len(line) {
			continue
		}
		b.WriteString(line[last:e.span.Start])
		b.WriteString(e.text)
		last = e.span.End
	}
	b.WriteString(line[last:])
	return b.String()
}

func hunk(n int, old, new string) string {
	return fmt.Sprintf("@@ -%d +%d @@\n-%s\n+%s\n", n, n, old, new)
}
