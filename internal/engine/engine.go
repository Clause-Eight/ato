// Package engine runs detectors over files and turns matches into findings.
package engine

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/schema"
	"github.com/Clause-Eight/pii-eu/internal/walk"
)

// maxLine caps how much of one line is read; the rest of a very long line is skipped.
const maxLine = 1 << 20

// Request describes one scan of a folder or file. SpecialCategories reports schema fields holding GDPR Art. 9
// data as findings; Inventory collects every personal-data field from schemas and models; Sinks reports log,
// error-tracker and analytics calls that are passed personal data.
type Request struct {
	Root              string
	Walk              walk.Options
	Detectors         []detectors.Detector
	Workers           int
	SpecialCategories bool
	Inventory         bool
	Sinks             bool
}

// Result is what a scan found, sorted by path, line and column.
type Result struct {
	Findings  []domain.Finding
	Inventory []domain.InventoryEntry
	Summary   domain.Summary
}

// Scan lists files, scans them in parallel and returns sorted findings.
func Scan(req Request) (Result, error) {
	files, skipped, err := walk.Files(req.Root, req.Walk)
	if err != nil {
		return Result{}, err
	}
	workers := req.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	type out struct {
		findings []domain.Finding
		entries  []domain.InventoryEntry
		skip     *domain.Skipped
		bad      *domain.Skipped
	}
	jobs := make(chan walk.File)
	results := make(chan out)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				fs, entries, skip, err := scanFile(f, req)
				o := out{findings: fs, entries: entries, skip: skip}
				if err != nil {
					o.bad = &domain.Skipped{Path: f.Rel, Reason: err.Error()}
				}
				results <- o
			}
		}()
	}
	go func() {
		for _, f := range files {
			jobs <- f
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	res := Result{Summary: domain.Summary{FilesSkipped: skipped}}
	for o := range results {
		switch {
		case o.bad != nil:
			res.Summary.Unreadable = append(res.Summary.Unreadable, *o.bad)
		case o.skip != nil:
			res.Summary.FilesSkipped = append(res.Summary.FilesSkipped, *o.skip)
		default:
			res.Summary.FilesScanned++
			res.Findings = append(res.Findings, o.findings...)
			if req.Inventory {
				res.Inventory = append(res.Inventory, o.entries...)
			}
		}
	}
	sortFindings(res.Findings)
	sortInventory(res.Inventory)
	sortSkipped(res.Summary.FilesSkipped)
	sortSkipped(res.Summary.Unreadable)
	return res, nil
}

func scanFile(f walk.File, req Request) ([]domain.Finding, []domain.InventoryEntry, *domain.Skipped, error) {
	fh, err := walk.Open(f)
	if err != nil {
		return nil, nil, nil, err
	}
	defer fh.Close()
	r := bufio.NewReaderSize(fh, 64<<10)
	head, _ := r.Peek(8000)
	if walk.IsBinary(head) {
		return nil, nil, &domain.Skipped{Path: f.Rel, Reason: "binary"}, nil
	}
	if !(req.SpecialCategories || req.Inventory) || !schema.Candidate(f.Rel) {
		fs, err := ScanText(f.Rel, r, req.Detectors, req.Sinks)
		return fs, nil, nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read error: %w", err)
	}
	fs, err := ScanText(f.Rel, bytes.NewReader(data), req.Detectors, req.Sinks)
	entries := inventory(f.Rel, data)
	if req.SpecialCategories {
		fs = append(fs, specialFindings(entries, splitLines(data))...)
	}
	return fs, entries, nil, err
}

// ScanReader scans text line by line for values. path is used for locations and for recognising test folders.
func ScanReader(path string, r io.Reader, dets []detectors.Detector) ([]domain.Finding, error) {
	return ScanText(path, r, dets, false)
}

// ScanText scans text line by line for values and, when sinks is set and path names source code, for log and
// telemetry calls that are passed personal data.
func ScanText(path string, r io.Reader, dets []detectors.Detector, sinks bool) ([]domain.Finding, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 64<<10)
	}
	var findings []domain.Finding
	ctx := lineContext{path: path, testPath: IsTestPath(path)}
	var calls *sinkScanner
	if sinks {
		calls = newSinkScanner(path)
	}
	for lineNo := 1; ; lineNo++ {
		line, err := readLine(br)
		if len(line) > 0 || err == nil {
			ctx.line, ctx.number = line, lineNo
			findings = append(findings, scanLine(ctx, dets)...)
			if calls != nil {
				findings = append(findings, calls.line(ctx)...)
			}
			ctx.previous = line
		}
		if errors.Is(err, io.EOF) {
			return findings, nil
		}
		if err != nil {
			return findings, fmt.Errorf("read error at line %d: %w", lineNo, err)
		}
	}
}

func readLine(r *bufio.Reader) (string, error) {
	var buf []byte
	for {
		chunk, isPrefix, err := r.ReadLine()
		if len(buf) < maxLine {
			buf = append(buf, chunk...)
		}
		if err != nil {
			return string(buf), err
		}
		if !isPrefix {
			return string(buf), nil
		}
	}
}

func sortFindings(fs []domain.Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i].Location, fs[j].Location
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return fs[i].Detector < fs[j].Detector
	})
}

func sortSkipped(s []domain.Skipped) {
	sort.Slice(s, func(i, j int) bool { return s[i].Path < s[j].Path })
}
