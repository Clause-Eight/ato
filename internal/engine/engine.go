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
	"strings"
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

// Request describes one scan of a folder, file or saved container image. SpecialCategories reports schema
// fields holding GDPR Art. 9 data as findings; Inventory collects every personal-data field from schemas,
// models and data file headers. MaxRows caps the rows read from each data file; 0 reads them all.
// Recipients collects the third-party services the code names; Sinks reports log, error-tracker and analytics
// calls that are passed personal data.
type Request struct {
	Root              string
	Walk              walk.Options
	Detectors         []detectors.Detector
	Workers           int
	SpecialCategories bool
	Inventory         bool
	MaxRows           int
	Sinks             bool
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

// fileResult is one scanned file or archive entry. Layer and name place an image entry for Shadowed.
type fileResult struct {
	findings []domain.Finding
	entries  []domain.InventoryEntry
	sampled  *domain.Skipped
	uses     []domain.RecipientUse
	evidence []lifecycle.Evidence
	tf       *infra.File
	layer    int
	name     string
}

// batch is what one job scanned: a file, every entry of an archive, or an image layer.
type batch struct {
	files      []fileResult
	skipped    []domain.Skipped
	unreadable []domain.Skipped
}

// add records the outcome of scanning one file. A file stopped by an archive limit keeps what was found.
func (b *batch) add(rel string, fr fileResult, skip *domain.Skipped, err error) {
	switch {
	case err != nil && !walk.IsLimit(err):
		b.unreadable = append(b.unreadable, domain.Skipped{Path: rel, Reason: err.Error()})
	case skip != nil:
		b.skipped = append(b.skipped, *skip)
	default:
		b.files = append(b.files, fr)
	}
}

// Scan lists files, scans them in parallel and returns sorted findings.
func Scan(req Request) (Result, error) {
	files, skipped, err := walk.Files(req.Root, req.Walk)
	if err != nil {
		return Result{}, err
	}
	batches := parallel(len(files), req.Workers, func(i int) batch { return scanFile(files[i], req) })
	batches = append(batches, batch{skipped: skipped})
	return fold(batches, req), nil
}

// ScanImage scans a container image saved with `docker save`, layer by layer in parallel. A file deleted or
// replaced by a later layer is still reported, since it is still in the image, and the finding says so.
func ScanImage(req Request) (Result, error) {
	img, err := walk.OpenImage(req.Root, req.Walk)
	if err != nil {
		return Result{}, err
	}
	defer img.Close()
	batches := parallel(len(img.Layers)+len(img.Configs), req.Workers, func(i int) batch {
		if i >= len(img.Layers) {
			return scanImageConfig(img, img.Configs[i-len(img.Layers)], req)
		}
		return scanLayer(img, i, req)
	})
	for _, b := range batches {
		for _, fr := range b.files {
			if fr.layer < 0 {
				continue
			}
			if why := img.Shadowed(fr.layer, fr.name); why != "" {
				for k := range fr.findings {
					fr.findings[k].Reasons = append(fr.findings[k].Reasons, why)
				}
			}
		}
	}
	return fold(batches, req), nil
}

func scanImageConfig(img *walk.Image, name string, req Request) batch {
	var b batch
	_ = img.VisitConfig(name, func(e walk.Entry, r io.Reader) error {
		fr, skip, err := scanContent(e.Rel, r, req)
		fr.layer = -1
		b.add(e.Rel, fr, skip, err)
		return nil
	})
	return b
}

func scanLayer(img *walk.Image, i int, req Request) batch {
	var b batch
	prefix := img.Rel + "!/" + img.Layers[i].ID + "/"
	out := img.VisitLayer(i, func(e walk.Entry, r io.Reader) error {
		fr, skip, err := scanContent(e.Rel, r, req)
		fr.layer = i
		fr.name, _, _ = strings.Cut(strings.TrimPrefix(e.Rel, prefix), "!/")
		b.add(e.Rel, fr, skip, err)
		return err
	})
	b.skipped = append(b.skipped, out.Skipped...)
	b.unreadable = append(b.unreadable, out.Unreadable...)
	return b
}

// parallel runs n jobs on the workers and returns their batches in job order.
func parallel(n, workers int, job func(int) batch) []batch {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	out := make([]batch, n)
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(workers, max(n, 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				out[i] = job(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

func fold(batches []batch, req Request) Result {
	var res Result
	var entries []domain.InventoryEntry
	var evidence []lifecycle.Evidence
	var tfFiles []*infra.File
	for _, b := range batches {
		res.Summary.FilesSkipped = append(res.Summary.FilesSkipped, b.skipped...)
		res.Summary.Unreadable = append(res.Summary.Unreadable, b.unreadable...)
		for _, fr := range b.files {
			res.Summary.FilesScanned++
			res.Findings = append(res.Findings, fr.findings...)
			entries = append(entries, fr.entries...)
			res.Recipients = append(res.Recipients, fr.uses...)
			evidence = append(evidence, fr.evidence...)
			if fr.tf != nil {
				tfFiles = append(tfFiles, fr.tf)
			}
			if fr.sampled != nil {
				res.Summary.Sampled = append(res.Summary.Sampled, *fr.sampled)
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
	sortSkipped(res.Summary.Sampled)
	return res
}

// scanFile scans one listed file, or every entry of an archive.
func scanFile(f walk.File, req Request) batch {
	var b batch
	fh, err := walk.Open(f)
	if err != nil {
		b.add(f.Rel, fileResult{}, nil, err)
		return b
	}
	defer fh.Close()
	if !walk.IsArchive(f.Rel) {
		fr, skip, err := scanContent(f.Rel, fh, req)
		b.add(f.Rel, fr, skip, err)
		return b
	}
	out, err := walk.VisitArchive(f.Rel, fh, req.Walk, func(e walk.Entry, r io.Reader) error {
		fr, skip, err := scanContent(e.Rel, r, req)
		b.add(e.Rel, fr, skip, err)
		return err
	})
	if err != nil {
		b.add(f.Rel, fileResult{}, nil, err)
	}
	b.skipped = append(b.skipped, out.Skipped...)
	b.unreadable = append(b.unreadable, out.Unreadable...)
	return b
}

// scanContent scans one file's content: data files by column, schemas and models also for their fields, other
// text line by line. Binary content is skipped.
func scanContent(rel string, content io.Reader, req Request) (fileResult, *domain.Skipped, error) {
	r := bufio.NewReaderSize(content, 64<<10)
	head, _ := r.Peek(8000)
	if walk.IsBinary(head) {
		if walk.Unopened(rel) {
			return fileResult{}, &domain.Skipped{Path: rel, Reason: "a document format pii-eu does not open", NotRead: true}, nil
		}
		return fileResult{}, &domain.Skipped{Path: rel, Reason: "binary"}, nil
	}
	wantFields := req.fields()
	if req.infra() && infra.Candidate(rel) {
		data, err := io.ReadAll(r)
		if err != nil {
			return fileResult{}, nil, fmt.Errorf("read error: %w", err)
		}
		tr, err := scanText(rel, bytes.NewReader(data), req.Detectors, 0, req.Sinks)
		return fileResult{findings: tr.findings, tf: infra.Parse(rel, data)}, nil, err
	}
	if walk.DataKind(rel) != "" {
		tr, err := scanText(rel, r, req.Detectors, req.MaxRows, false)
		fr := fileResult{findings: tr.findings}
		if tr.sampled {
			fr.sampled = &domain.Skipped{Path: rel, Reason: fmt.Sprintf("only the first %d rows were read (max_rows)", req.MaxRows), NotRead: true}
		}
		if wantFields {
			fr.entries = classifyFields(tr.fields)
			if req.SpecialCategories {
				fr.findings = append(fr.findings, specialFindings(fr.entries, nil)...)
			}
		}
		return fr, nil, err
	}
	wantSchema := wantFields && schema.Candidate(rel)
	wantRecipients := req.Recipients && recipients.Candidate(rel)
	wantLifecycle := req.Lifecycle && lifecycle.Candidate(rel)
	if !wantSchema && !wantRecipients && !wantLifecycle {
		tr, err := scanText(rel, r, req.Detectors, 0, req.Sinks)
		return fileResult{findings: tr.findings}, nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fileResult{}, nil, fmt.Errorf("read error: %w", err)
	}
	tr, err := scanText(rel, bytes.NewReader(data), req.Detectors, 0, req.Sinks)
	fr := fileResult{findings: tr.findings}
	if wantSchema {
		fr.entries = inventory(rel, data)
		if req.SpecialCategories {
			fr.findings = append(fr.findings, specialFindings(fr.entries, splitLines(data))...)
		}
	}
	if wantRecipients {
		fr.uses = recipients.Find(rel, data)
	}
	if wantLifecycle {
		fr.evidence = lifecycle.Find(rel, data)
	}
	return fr, nil, err
}

// ScanReader scans text line by line. path is used for locations, for recognising test folders and for
// reading data files and SQL dumps by column.
func ScanReader(path string, r io.Reader, dets []detectors.Detector) ([]domain.Finding, error) {
	tr, err := scanText(path, r, dets, 0, false)
	return tr.findings, err
}

type textResult struct {
	findings []domain.Finding
	fields   []domain.Field
	sampled  bool
}

// scanText scans lines, split into columns where the format has them. With maxRows above zero it stops after
// that many data rows and reports the file as sampled when more followed.
func scanText(path string, r io.Reader, dets []detectors.Detector, maxRows int, sinks bool) (textResult, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 64<<10)
	}
	var res textResult
	sp := splitterFor(path)
	ctx := lineContext{path: path, testPath: IsTestPath(path)}
	rows := 0
	var calls *sinkScanner
	if sinks {
		calls = newSinkScanner(path)
	}
	for lineNo := 1; ; lineNo++ {
		line, err := readLine(br)
		if len(line) > 0 || err == nil {
			if maxRows > 0 && rows >= maxRows && strings.TrimSpace(line) != "" {
				res.sampled = true
				break
			}
			ctx.line, ctx.number, ctx.segments = line, lineNo, nil
			if sp != nil {
				var row bool
				ctx.segments, row = sp.split(line, lineNo)
				if row {
					rows++
				}
			}
			res.findings = append(res.findings, scanLine(ctx, dets)...)
			if calls != nil {
				res.findings = append(res.findings, calls.line(ctx)...)
			}
			ctx.previous = line
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if sp != nil {
				res.fields = sp.fields()
			}
			return res, fmt.Errorf("read error at line %d: %w", lineNo, err)
		}
	}
	if sp != nil {
		res.fields = sp.fields()
	}
	return res, nil
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

// ScanText scans text line by line for values and, when sinks is set and path names source code, for log and
// telemetry calls that are passed personal data.
func ScanText(path string, r io.Reader, dets []detectors.Detector, sinks bool) ([]domain.Finding, error) {
	tr, err := scanText(path, r, dets, 0, sinks)
	return tr.findings, err
}
