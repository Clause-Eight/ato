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
	"github.com/Clause-Eight/pii-eu/internal/lifecycle"
	"github.com/Clause-Eight/pii-eu/internal/recipients"
	"github.com/Clause-Eight/pii-eu/internal/schema"
	"github.com/Clause-Eight/pii-eu/internal/walk"
)

// maxLine caps how much of one line is read; the rest of a very long line is skipped.
const maxLine = 1 << 20

// Request describes one scan of a folder or file. SpecialCategories reports schema fields holding GDPR Art. 9
// data as findings; Inventory collects every personal-data field from schemas and models.
type Request struct {
	Root              string
	Walk              walk.Options
	Detectors         []detectors.Detector
	Workers           int
	SpecialCategories bool
	Inventory         bool
	Recipients        bool
	// Lifecycle collects how each store of personal data is erased and expired; ErasureFindings and
	// RetentionFindings report stores without either.
	Lifecycle         bool
	ErasureFindings   bool
	RetentionFindings bool
	// Infra names the checks on Terraform data stores to report, such as tf-unencrypted.
	Infra []string
}

func (r Request) infra() bool { return r.Inventory || r.Lifecycle || len(r.Infra) > 0 }
func (r Request) fields() bool {
	return r.SpecialCategories || r.Inventory || r.Lifecycle || len(r.Infra) > 0
}

// Result is what a scan found, sorted by path, line and column.
type Result struct {
	Findings   []domain.Finding
	Inventory  []domain.InventoryEntry
	Recipients []domain.RecipientUse
	Lifecycle  []domain.StoreLifecycle
	// Stores are the data stores declared in Terraform, set with Inventory or Lifecycle.
	Stores  []domain.Store
	Summary domain.Summary
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
		uses     []domain.RecipientUse
		evidence []lifecycle.Evidence
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
				fs, sf, skip, err := scanFile(f, req)
				o := out{findings: fs, entries: sf.entries, uses: sf.uses, evidence: sf.evidence, tf: sf.tf, skip: skip}
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
	var evidence []lifecycle.Evidence
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
			res.Recipients = append(res.Recipients, o.uses...)
			evidence = append(evidence, o.evidence...)
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
		if req.Inventory || req.Lifecycle {
			res.Stores = stores
		}
	}
	if req.Lifecycle {
		res.Lifecycle = assessLifecycle(entries, evidence)
		res.Findings = append(res.Findings, lifecycleFindings(entries, res.Lifecycle, req)...)
	}
	sortFindings(res.Findings)
	sortRecipients(res.Recipients)
	sortSkipped(res.Summary.FilesSkipped)
	sortSkipped(res.Summary.Unreadable)
	return res, nil
}

// fileFacts is what one file adds besides findings.
type fileFacts struct {
	entries  []domain.InventoryEntry
	uses     []domain.RecipientUse
	evidence []lifecycle.Evidence
	tf       *infra.File
}

func scanFile(f walk.File, req Request) ([]domain.Finding, fileFacts, *domain.Skipped, error) {
	var facts fileFacts
	fh, err := walk.Open(f)
	if err != nil {
		return nil, facts, nil, err
	}
	defer fh.Close()
	r := bufio.NewReaderSize(fh, 64<<10)
	head, _ := r.Peek(8000)
	if walk.IsBinary(head) {
		return nil, facts, &domain.Skipped{Path: f.Rel, Reason: "binary"}, nil
	}
	if req.infra() && infra.Candidate(f.Rel) {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, facts, nil, fmt.Errorf("read error: %w", err)
		}
		fs, err := ScanReader(f.Rel, bytes.NewReader(data), req.Detectors)
		facts.tf = infra.Parse(f.Rel, data)
		return fs, facts, nil, err
	}
	wantSchema := req.fields() && schema.Candidate(f.Rel)
	wantRecipients := req.Recipients && recipients.Candidate(f.Rel)
	wantLifecycle := req.Lifecycle && lifecycle.Candidate(f.Rel)
	if !wantSchema && !wantRecipients && !wantLifecycle {
		fs, err := ScanReader(f.Rel, r, req.Detectors)
		return fs, facts, nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, facts, nil, fmt.Errorf("read error: %w", err)
	}
	fs, err := ScanReader(f.Rel, bytes.NewReader(data), req.Detectors)
	if wantSchema {
		facts.entries = inventory(f.Rel, data)
		if req.SpecialCategories {
			fs = append(fs, specialFindings(facts.entries, splitLines(data))...)
		}
	}
	if wantRecipients {
		facts.uses = recipients.Find(f.Rel, data)
	}
	if wantLifecycle {
		facts.evidence = lifecycle.Find(f.Rel, data)
	}
	return fs, facts, nil, err
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

func sortRecipients(us []domain.RecipientUse) {
	sort.SliceStable(us, func(i, j int) bool {
		a, b := us[i].Location, us[j].Location
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return us[i].Name < us[j].Name
	})
}
