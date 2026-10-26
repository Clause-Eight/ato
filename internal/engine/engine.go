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
	"github.com/Clause-Eight/pii-eu/internal/infra"
	"github.com/Clause-Eight/pii-eu/internal/schema"
	"github.com/Clause-Eight/pii-eu/internal/walk"
)

// maxLine caps how much of one line is read; the rest of a very long line is skipped.
const maxLine = 1 << 20

// Request describes one scan of a folder or file. SpecialCategories reports schema fields holding GDPR Art. 9
// data as findings; Inventory collects every personal-data field from schemas and models, and the data stores
// declared in Terraform. Infra names the checks on Terraform data stores to report, such as tf-unencrypted.
type Request struct {
	Root              string
	Walk              walk.Options
	Detectors         []detectors.Detector
	Workers           int
	SpecialCategories bool
	Inventory         bool
	Infra             []string
}

// Result is what a scan found, sorted by path, line and column. Stores is set with Inventory.
type Result struct {
	Findings  []domain.Finding
	Inventory []domain.InventoryEntry
	Stores    []domain.Store
	Summary   domain.Summary
}

func (r Request) infra() bool  { return r.Inventory || len(r.Infra) > 0 }
func (r Request) fields() bool { return r.SpecialCategories || r.Inventory || len(r.Infra) > 0 }

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
		tf       *infra.File
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
				fs, entries, tf, skip, err := scanFile(f, req)
				o := out{findings: fs, entries: entries, tf: tf, skip: skip}
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
	var entries []domain.InventoryEntry
	var tfFiles []*infra.File
	for o := range results {
		switch {
		case o.bad != nil:
			res.Summary.Unreadable = append(res.Summary.Unreadable, *o.bad)
		case o.skip != nil:
			res.Summary.FilesSkipped = append(res.Summary.FilesSkipped, *o.skip)
		default:
			res.Summary.FilesScanned++
			res.Findings = append(res.Findings, o.findings...)
			entries = append(entries, o.entries...)
			if o.tf != nil {
				tfFiles = append(tfFiles, o.tf)
			}
		}
	}
	sortInventory(entries)
	if req.Inventory {
		res.Inventory = entries
	}
	if len(tfFiles) > 0 {
		stores := infra.Stores(tfFiles)
		linkStores(stores, entries)
		res.Findings = append(res.Findings, storeFindings(stores, req.Infra, tfFiles)...)
		if req.Inventory {
			res.Stores = stores
		}
	}
	sortFindings(res.Findings)
	sortSkipped(res.Summary.FilesSkipped)
	sortSkipped(res.Summary.Unreadable)
	return res, nil
}

func scanFile(f walk.File, req Request) ([]domain.Finding, []domain.InventoryEntry, *infra.File, *domain.Skipped, error) {
	fh, err := walk.Open(f)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer fh.Close()
	r := bufio.NewReaderSize(fh, 64<<10)
	head, _ := r.Peek(8000)
	if walk.IsBinary(head) {
		return nil, nil, nil, &domain.Skipped{Path: f.Rel, Reason: "binary"}, nil
	}
	isSchema := req.fields() && schema.Candidate(f.Rel)
	isInfra := req.infra() && infra.Candidate(f.Rel)
	if !isSchema && !isInfra {
		fs, err := ScanReader(f.Rel, r, req.Detectors)
		return fs, nil, nil, nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read error: %w", err)
	}
	fs, err := ScanReader(f.Rel, bytes.NewReader(data), req.Detectors)
	if isInfra {
		return fs, nil, infra.Parse(f.Rel, data), nil, err
	}
	entries := inventory(f.Rel, data)
	if req.SpecialCategories {
		fs = append(fs, specialFindings(entries, splitLines(data))...)
	}
	return fs, entries, nil, nil, err
}

// ScanReader scans text line by line. path is used for locations and for recognising test folders.
func ScanReader(path string, r io.Reader, dets []detectors.Detector) ([]domain.Finding, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 64<<10)
	}
	var findings []domain.Finding
	ctx := lineContext{path: path, testPath: IsTestPath(path)}
	for lineNo := 1; ; lineNo++ {
		line, err := readLine(br)
		if len(line) > 0 || err == nil {
			ctx.line, ctx.number = line, lineNo
			findings = append(findings, scanLine(ctx, dets)...)
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
