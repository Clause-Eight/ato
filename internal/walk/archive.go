package walk

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync/atomic"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Limits protect against archive bombs. They apply to one archive or image with everything nested in it. A
// zero field takes the default.
type Limits struct {
	// MaxBytes caps the bytes produced by decompression.
	MaxBytes int64
	// MaxEntries caps the entries read from tar files.
	MaxEntries int64
	// MaxDepth caps how many archives deep a file may sit: 1 opens an archive but no archive inside it.
	MaxDepth int
}

// Default limits.
const (
	DefaultMaxArchiveBytes   = 4 << 30
	DefaultMaxArchiveEntries = 1_000_000
	DefaultMaxArchiveDepth   = 3
)

func (l Limits) withDefaults() Limits {
	if l.MaxBytes <= 0 {
		l.MaxBytes = DefaultMaxArchiveBytes
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = DefaultMaxArchiveEntries
	}
	if l.MaxDepth <= 0 {
		l.MaxDepth = DefaultMaxArchiveDepth
	}
	return l
}

// LimitError stops reading an archive whose size or entry count passed a limit.
type LimitError struct{ Reason string }

func (e *LimitError) Error() string { return e.Reason }

// IsLimit reports whether err stopped an archive at a limit.
func IsLimit(err error) bool {
	var le *LimitError
	return errors.As(err, &le)
}

// Entry is one file inside an archive or image. Rel is the path shown in reports, such as
// `dump.tar.gz!/data/users.csv`; Name is the path inside its archive. Size is -1 when unknown.
type Entry struct {
	Rel  string
	Name string
	Size int64
}

// VisitFunc scans one entry. Returning an error from IsLimit stops the archive; other errors are the
// caller's to record, and reading continues.
type VisitFunc func(Entry, io.Reader) error

// Outcome is what an archive visit could not scan.
type Outcome struct {
	Skipped    []domain.Skipped
	Unreadable []domain.Skipped
}

// IsArchive reports whether a file name is a gzip, tar or zip file, which is scanned inside.
func IsArchive(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".tar") || strings.HasSuffix(n, ".tgz") || strings.HasSuffix(n, ".gz") || strings.HasSuffix(n, ".zip")
}

// unopened are formats that often hold personal data and that ATO does not open: office documents, PDFs and
// compression formats other than gzip, tar and zip. Skipping one makes a scan incomplete.
var unopened = []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".xlsm", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".rtf",
	".7z", ".rar", ".bz2", ".xz", ".zst", ".lz4", ".sqlite", ".db", ".mdb", ".accdb", ".parquet", ".avro"}

// Unopened reports whether a file is in a format ATO does not open but that often holds personal data.
func Unopened(name string) bool {
	n := strings.ToLower(name)
	for _, ext := range unopened {
		if strings.HasSuffix(n, ext) {
			return true
		}
	}
	return false
}

// VisitArchive reads the archive rel from r and calls fn for every regular file inside, opening nested
// archives up to the depth limit. Excludes apply to paths inside the archive; includes selected the archive.
func VisitArchive(rel string, r io.Reader, opts Options, fn VisitFunc) (Outcome, error) {
	exc, err := compileAll(opts.Exclude)
	if err != nil {
		return Outcome{}, err
	}
	v := &visitor{fn: fn, exc: exc, maxFile: opts.MaxFileSize, budget: newBudget(opts.Limits)}
	if err := v.archive(rel, path.Base(rel), r, 1); err != nil {
		v.out.Unreadable = append(v.out.Unreadable, domain.Skipped{Path: rel, Reason: err.Error()})
	}
	return v.out, nil
}

type budget struct {
	lim     Limits
	bytes   atomic.Int64
	entries atomic.Int64
}

func newBudget(l Limits) *budget { return &budget{lim: l.withDefaults()} }

func (b *budget) entry() error {
	if b.entries.Add(1) > b.lim.MaxEntries {
		return &LimitError{fmt.Sprintf("stopped after %d entries (max_archive_entries); the rest was not scanned", b.lim.MaxEntries)}
	}
	return nil
}

// counted counts the bytes a decompressor produces against the budget.
type counted struct {
	r io.Reader
	b *budget
}

func (c counted) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if c.b.bytes.Add(int64(n)) > c.b.lim.MaxBytes {
		return n, &LimitError{fmt.Sprintf("stopped after %d uncompressed bytes (max_archive_bytes); the rest was not scanned", c.b.lim.MaxBytes)}
	}
	return n, err
}

type visitor struct {
	fn      VisitFunc
	exc     []glob
	maxFile int64
	budget  *budget
	out     Outcome
}

// archive reads one gzip, tar or zip file at the given depth.
func (v *visitor) archive(rel, name string, r io.Reader, depth int) error {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".zip") {
		return v.zip(rel, r, depth)
	}
	if !strings.HasSuffix(lower, ".gz") && !strings.HasSuffix(lower, ".tgz") {
		return v.tar(rel, r, depth)
	}
	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer zr.Close()
	inner := strings.TrimSuffix(strings.TrimSuffix(name, path.Ext(name)), ".")
	if strings.HasSuffix(lower, ".tgz") {
		inner += ".tar"
	}
	br := bufio.NewReaderSize(counted{zr, v.budget}, 64<<10)
	if head, _ := br.Peek(512); strings.HasSuffix(strings.ToLower(inner), ".tar") || isTar(head) {
		return v.tar(rel, br, depth)
	}
	return v.entry(rel+"!/"+inner, inner, br, -1, depth)
}

// isTar recognises a POSIX or GNU tar header by its magic.
func isTar(head []byte) bool {
	return len(head) >= 262 && bytes.Equal(head[257:262], []byte("ustar"))
}

func (v *visitor) tar(rel string, r io.Reader, depth int) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if IsLimit(err) {
				return err
			}
			return fmt.Errorf("not a valid tar file: %w", err)
		}
		if err := v.budget.entry(); err != nil {
			return err
		}
		name := cleanName(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || name == "" || matchAny(v.exc, name) {
			continue
		}
		if err := v.entry(rel+"!/"+name, name, tr, hdr.Size, depth); err != nil {
			return err
		}
	}
}

// zip reads a zip file. Zip needs random access, so the file is read into memory first, within the same byte
// budget as everything an archive unpacks; entries count against the entry and byte limits like tar members.
func (v *visitor) zip(rel string, r io.Reader, depth int) error {
	data, err := io.ReadAll(counted{r, v.budget})
	if err != nil {
		if IsLimit(err) {
			return err
		}
		return fmt.Errorf("read error: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("not a valid zip file: %w", err)
	}
	for _, f := range zr.File {
		if err := v.budget.entry(); err != nil {
			return err
		}
		name := cleanName(f.Name)
		if f.FileInfo().IsDir() || !f.Mode().IsRegular() || name == "" || matchAny(v.exc, name) {
			continue
		}
		member := rel + "!/" + name
		if f.Flags&0x1 != 0 {
			v.out.Unreadable = append(v.out.Unreadable, domain.Skipped{Path: member, Reason: "encrypted zip entry"})
			continue
		}
		rc, err := f.Open()
		if err != nil {
			v.out.Unreadable = append(v.out.Unreadable, domain.Skipped{Path: member, Reason: "cannot open: " + err.Error()})
			continue
		}
		err = v.entry(member, name, counted{rc, v.budget}, int64(f.UncompressedSize64), depth)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// entry hands one file to fn, or opens it when it is an archive itself. Only limit errors are returned.
func (v *visitor) entry(rel, name string, r io.Reader, size int64, depth int) error {
	if IsArchive(name) {
		if depth >= v.budget.lim.MaxDepth {
			v.out.Skipped = append(v.out.Skipped, domain.Skipped{Path: rel, Reason: fmt.Sprintf("archive nested more than %d deep (max_archive_depth)", v.budget.lim.MaxDepth), NotRead: true})
			return nil
		}
		err := v.archive(rel, path.Base(name), r, depth+1)
		if err != nil && !IsLimit(err) {
			v.out.Unreadable = append(v.out.Unreadable, domain.Skipped{Path: rel, Reason: err.Error()})
			return nil
		}
		return err
	}
	if v.maxFile > 0 && DataKind(name) == "" {
		if size > v.maxFile {
			v.out.Skipped = append(v.out.Skipped, domain.Skipped{Path: rel, Reason: fmt.Sprintf("larger than %d bytes", v.maxFile), NotRead: true})
			return nil
		}
		if size < 0 {
			data, err := io.ReadAll(io.LimitReader(r, v.maxFile+1))
			if err != nil {
				return v.readError(rel, err)
			}
			if int64(len(data)) > v.maxFile {
				v.out.Skipped = append(v.out.Skipped, domain.Skipped{Path: rel, Reason: fmt.Sprintf("larger than %d bytes", v.maxFile), NotRead: true})
				return nil
			}
			r, size = bytes.NewReader(data), int64(len(data))
		}
	}
	err := v.fn(Entry{Rel: rel, Name: name, Size: size}, r)
	if IsLimit(err) {
		return err
	}
	return nil
}

func (v *visitor) readError(rel string, err error) error {
	if IsLimit(err) {
		return err
	}
	v.out.Unreadable = append(v.out.Unreadable, domain.Skipped{Path: rel, Reason: "read error: " + err.Error()})
	return nil
}

// cleanName turns a tar member name into a relative slash path.
func cleanName(name string) string {
	n := path.Clean("/" + strings.ReplaceAll(name, "\\", "/"))
	return strings.TrimPrefix(n, "/")
}
