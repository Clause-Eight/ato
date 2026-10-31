package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/Clause-Eight/ato/v2/internal/binmeta"
	"github.com/Clause-Eight/ato/v2/internal/classify"
	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Detectors for files that hold personal data as a whole or in their metadata (ADR 13). Each can be switched
// off in settings like a detector.
const (
	DumpDetector     = "database-dump"
	AuthorDetector   = "document-author"
	PhotoGPSDetector = "photo-location"
	IDScanDetector   = "id-document-scan"
)

// FileDetectors are the names of the file-level detectors.
func FileDetectors() []string {
	return []string{DumpDetector, AuthorDetector, PhotoGPSDetector, IDScanDetector}
}

func (r Request) file(name string) bool { return slices.Contains(r.Files, name) }

// How much of a binary file is read. A JPEG keeps its EXIF in the first segments; other formats are read whole up
// to the cap, and a larger file is left as it was before these readers existed.
const (
	maxJPEGHead   = 256 << 10
	maxBinaryRead = 32 << 20
)

// Binary formats read for metadata.
const (
	formatPGDump = "pg_dump"
	formatOffice = "office"
	formatPDF    = "pdf"
	formatImage  = "image"
)

var officeExts = map[string]bool{".docx": true, ".docm": true, ".dotx": true, ".xlsx": true, ".xlsm": true, ".xltx": true,
	".pptx": true, ".pptm": true, ".potx": true, ".ppsx": true, ".odt": true, ".ods": true, ".odp": true, ".odg": true}

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".jpe": true, ".tif": true, ".tiff": true, ".png": true,
	".webp": true, ".heic": true, ".heif": true, ".avif": true, ".dng": true}

// binaryFormat names the binary format a file is read as for metadata, or "" to scan it as before. A format is
// only read when a detector that uses it is on.
func binaryFormat(rel string, head []byte, req Request) string {
	ext := strings.ToLower(path.Ext(rel))
	switch {
	case binmeta.IsPGDump(head):
		if req.file(DumpDetector) {
			return formatPGDump
		}
	case officeExts[ext] && bytes.HasPrefix(head, []byte("PK\x03\x04")):
		if req.file(AuthorDetector) {
			return formatOffice
		}
	case bytes.HasPrefix(head, []byte("%PDF-")):
		if len(mrzDetectors(req.Detectors)) > 0 {
			return formatPDF
		}
	case imageExts[ext]:
		if req.file(PhotoGPSDetector) {
			return formatImage
		}
	}
	return ""
}

func mrzDetectors(dets []detectors.Detector) []detectors.Detector {
	var out []detectors.Detector
	for _, d := range dets {
		if d.Name == "mrz" {
			out = append(out, d)
		}
	}
	return out
}

// notOpened is the reason for office documents and PDFs, whose body ATO does not read (ADR 8).
const notOpened = "a document format ATO does not open"

// scanBinary reads the metadata of a binary file. Office documents, PDFs and pg_dump archives stay not read
// (ADR 8): their body or rows were not scanned, only what is named in the reason.
func scanBinary(format, rel string, r *bufio.Reader, req Request) (fileResult, *domain.Skipped, error) {
	limit := int64(maxBinaryRead)
	if format == formatImage && bytes.HasPrefix(peek(r, 2), []byte{0xff, 0xd8}) {
		limit = maxJPEGHead
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return fileResult{}, nil, fmt.Errorf("read error: %w", err)
	}
	truncated := int64(len(data)) > limit
	notRead := func(reason string) *domain.Skipped {
		return &domain.Skipped{Path: rel, Reason: reason, NotRead: true}
	}
	switch format {
	case formatPGDump:
		fr := fileResult{findings: pgDumpFindings(rel, binmeta.PGDumpTables(data))}
		fr.notRead = notRead("a pg_dump archive; only its table of contents was read, not its rows")
		return fr, nil, nil
	case formatOffice:
		if truncated {
			return fileResult{}, notRead(notOpened), nil
		}
		authors, err := binmeta.OfficeAuthors(data)
		if err != nil {
			return fileResult{}, notRead(notOpened), nil
		}
		return fileResult{findings: authorFindings(rel, authors), notRead: notRead(notOpened + "; only its author metadata was read")}, nil, nil
	case formatPDF:
		lines := binmeta.PDFText(data)
		var fs []domain.Finding
		if len(lines) > 0 {
			tr, _ := scanText(rel, strings.NewReader(strings.Join(lines, "\n")), mrzDetectors(req.Detectors), 0, false)
			fs = pdfFindings(tr.findings)
		}
		return fileResult{findings: fs, notRead: notRead(notOpened + "; its text was searched only for machine readable zones")}, nil, nil
	}
	if g, ok := binmeta.ImageGPS(data); ok {
		return fileResult{findings: []domain.Finding{gpsFinding(rel, g)}}, nil, nil
	}
	return fileResult{}, &domain.Skipped{Path: rel, Reason: "binary"}, nil
}

func peek(r *bufio.Reader, n int) []byte {
	b, _ := r.Peek(n)
	return b
}

// fileFinding fills what every file-level finding shares. It sits at the start of the file unless line says
// otherwise, and a test or fixture path lowers its confidence.
func fileFinding(detector, label, rel string, line int, category domain.DataCategory, masked string, conf domain.Confidence, reasons []string, shape string) domain.Finding {
	f := domain.Finding{
		Kind:        domain.KindFile,
		Category:    category,
		Detector:    detector,
		Label:       label,
		Country:     domain.CrossBorder,
		Location:    domain.Location{Path: rel, Line: max(line, 1), Column: 1},
		Masked:      masked,
		Confidence:  conf,
		Reasons:     reasons,
		TestPath:    IsTestPath(rel),
		Fingerprint: fingerprint(detector, rel, shape, 0),
	}
	if f.TestPath {
		f.Confidence = lower1(f.Confidence)
		f.Reasons = append(f.Reasons, "in a test, fixture or example path")
	}
	return f
}

const dumpLabel = "Database dump with rows of personal data"

// personalColumns keeps the columns whose names say they hold personal data, and the category of the first.
func personalColumns(cols []string) ([]string, domain.DataCategory) {
	var out []string
	var cat domain.DataCategory
	for _, c := range cols {
		if cl, ok := classify.Field(c); ok {
			if cat == "" {
				cat = cl.Category
			}
			out = append(out, c)
		}
	}
	return out, cat
}

// dumpFindings reports each table of a SQL dump that received rows into columns of personal data. A file that
// says which tool wrote it, or many rows, is surely a dump; a few rows without that are more likely seed data.
func dumpFindings(rel string, d *sqlDump) []domain.Finding {
	var out []domain.Finding
	for _, name := range d.order {
		t := d.dumped[name]
		cols, cat := personalColumns(t.cols)
		if t.rows == 0 || len(cols) == 0 {
			continue
		}
		conf := domain.Low
		reasons := []string{fmt.Sprintf("%d %s inserted into %s", t.rows, plural(t.rows, "row", "rows"), name)}
		switch {
		case d.tool != "" && t.rows >= 1:
			conf = domain.High
			reasons = append(reasons, "written by "+d.tool)
		case t.rows >= 20:
			conf = domain.Medium
		}
		reasons = append(reasons, "columns holding personal data: "+strings.Join(cols, ", "))
		out = append(out, fileFinding(DumpDetector, dumpLabel, rel, t.line, cat, name+": "+strings.Join(cols, ", "), conf, reasons, name))
	}
	return out
}

func pgDumpFindings(rel string, tables []binmeta.DumpTable) []domain.Finding {
	var out []domain.Finding
	for _, t := range tables {
		cols, cat := personalColumns(t.Columns)
		if len(cols) == 0 {
			continue
		}
		reasons := []string{"pg_dump custom-format archive with data for " + t.Name, "columns holding personal data: " + strings.Join(cols, ", "),
			"the rows are compressed and were not read"}
		out = append(out, fileFinding(DumpDetector, dumpLabel, rel, 1, cat, t.Name+": "+strings.Join(cols, ", "), domain.Medium, reasons, t.Name))
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// authorFindings reports author properties that hold a person's name: two to four capitalised words, medium
// confidence when every word is in the name list and low when at least one is. Tool names, generic accounts
// ("Microsoft Office User") and user names are left out.
func authorFindings(rel string, authors []binmeta.Author) []domain.Finding {
	var out []domain.Finding
	for i, a := range authors {
		words := strings.FieldsFunc(a.Name, func(r rune) bool { return r == ' ' || r == '-' })
		if len(words) < 2 || len(words) > 4 {
			continue
		}
		known, ok := 0, true
		for _, w := range words {
			ok = ok && nameWord(w)
			if detectors.KnownName(w) {
				known++
			}
		}
		if !ok || known == 0 {
			continue
		}
		conf := domain.Low
		if known == len(words) {
			conf = domain.Medium
		}
		reasons := []string{"a person's name in the document's " + a.Property}
		out = append(out, fileFinding(AuthorDetector, "Person name in document metadata (author)", rel, 1, domain.CategoryName,
			maskWords(a.Name), conf, reasons, fmt.Sprintf("%s\x00%d", a.Property, i)))
	}
	return out
}

// nameWord is a capitalised word of letters, with an apostrophe allowed after the first letter.
func nameWord(w string) bool {
	for i, r := range w {
		switch {
		case i == 0 && !unicode.IsUpper(r):
			return false
		case !unicode.IsLetter(r) && !(i > 0 && (r == '\'' || r == '’')):
			return false
		}
	}
	return w != ""
}

// maskWords hides every letter and keeps spaces and hyphens, so a reviewer sees the shape of the name only.
func maskWords(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return r
		}
		return '•'
	}, s)
}

// gpsMask stands for every position: neither the digits, their count nor the hemisphere is shown.
const gpsMask = "••.••••, •••.••••"

// gpsFinding reports a photo with a position. The position itself is only checked for range and never kept.
func gpsFinding(rel string, _ binmeta.GPS) domain.Finding {
	return fileFinding(PhotoGPSDetector, "Photo with a GPS position in its EXIF metadata", rel, 1, domain.CategoryLocation, gpsMask, domain.Medium,
		[]string{"EXIF GPSLatitude and GPSLongitude are set", "a photo's position often shows where a person lives or was"}, "")
}

// pdfFindings turns MRZ lines found in a PDF's text into file findings: the line numbers are those of the
// extracted text, not of the file, and the value cannot be replaced in place.
func pdfFindings(fs []domain.Finding) []domain.Finding {
	for i := range fs {
		fs[i].Kind = domain.KindFile
		fs[i].Location.Line, fs[i].Location.Column = 1, 1
		fs[i].Span = domain.Span{}
		fs[i].Reasons = append(fs[i].Reasons, "in text extracted from the PDF")
	}
	return fs
}

// idScanExts are the formats an identity document is scanned or photographed to.
var idScanExts = map[string]bool{".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".heic": true, ".heif": true,
	".tif": true, ".tiff": true, ".webp": true, ".bmp": true, ".gif": true}

// idScanWords name identity documents in file names, in the EU languages. Words are folded and written with
// spaces between the parts of the name; a file name matches them with or without separators. Weak words also
// mean something else (Polish "dni" is "days", Romanian "buletin" is also a newsletter) and give low confidence.
var idScanWords = []string{
	"passport", "id card", "identity card", "national id card", "id scan", "id document", "identity document",
	"driving licence", "driver license", "drivers license", "residence permit",
	"passeport", "carte identite", "carte d identite", "permis de conduire", "titre de sejour",
	"reisepass", "personalausweis", "fuhrerschein", "fuehrerschein", "aufenthaltstitel",
	"pasaporte", "documento de identidad", "documento identidad", "carnet de conducir",
	"passaporto", "carta identita", "carta d identita", "carta di identita", "patente di guida", "permesso di soggiorno",
	"passaporte", "cartao de cidadao", "cartao cidadao", "bilhete de identidade",
	"pasaport", "carte de identitate", "permis de conducere",
	"paszport", "dowod osobisty", "prawo jazdy",
	"paspoort", "identiteitskaart", "identiteitsbewijs", "rijbewijs",
	"obcansky prukaz", "obciansky preukaz", "cestovni pas", "cestovny pas", "ridicsky prukaz",
	"szemelyi igazolvany", "utlevel", "jogositvany",
	"osobna iskaznica", "osebna izkaznica", "putovnica", "potni list", "vozacka dozvola", "vozniska dovoljenje",
	"henkilokortti", "ajokortti", "isikutunnistus", "juhiluba", "id kaart",
	"personas apliecība", "asmens tapatybes kortele", "asmens kortele",
	"korekort", "korkort", "id kort", "karta tal identita",
	"лична карта", "паспорт", "διαβατηριο", "ταυτοτητα",
}

var idScanWeak = []string{"dni", "cni", "buletin", "szemelyi", "ausweis", "pase", "pasas", "passi", "legitimation"}

var idScanIndex = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range idScanWords {
		m[strings.ReplaceAll(detectors.Fold(w), " ", "")] = false
	}
	for _, w := range idScanWeak {
		m[detectors.Fold(w)] = true
	}
	return m
}()

// idScanFinding reports an image or PDF whose name says it is a scan or photo of an identity document. Nothing
// in the file is read: ATO does no OCR. A word in the file name gives medium confidence, a weak word or a word
// in a folder name low.
func idScanFinding(rel string) (domain.Finding, bool) {
	if !idScanExts[strings.ToLower(path.Ext(rel))] {
		return domain.Finding{}, false
	}
	segs := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '!' || r == '\\' })
	for i := len(segs) - 1; i >= 0; i-- {
		seg := segs[i]
		if i == len(segs)-1 {
			seg = strings.TrimSuffix(seg, path.Ext(seg))
		}
		word, weak, ok := idScanMatch(nameTokens(seg))
		if !ok {
			continue
		}
		conf, where := domain.Medium, "file name"
		if i < len(segs)-1 {
			where = "folder name"
		}
		if weak || i < len(segs)-1 {
			conf = domain.Low
		}
		reasons := []string{fmt.Sprintf("the %s says it is an identity document (%q)", where, word), "the content was not read: ATO does no OCR"}
		return fileFinding(IDScanDetector, "Possible scan of an identity document", rel, 1, domain.CategoryIdentifier, word, conf, reasons, word), true
	}
	return domain.Finding{}, false
}

// idScanMatch finds an identity-document word in a name's tokens, written as one token or several in a row, or
// with a plural s.
func idScanMatch(toks []string) (string, bool, bool) {
	for i := range toks {
		joined := ""
		for j := i; j < len(toks) && j < i+5; j++ {
			joined += toks[j]
			weak, ok := idScanIndex[joined]
			if !ok && len(joined) > 6 {
				weak, ok = idScanIndex[strings.TrimSuffix(joined, "s")]
			}
			if ok {
				return strings.Join(toks[i:j+1], " "), weak, true
			}
		}
	}
	return "", false, false
}

// nameTokens splits a file or folder name into folded words at separators, digits and camelCase boundaries.
func nameTokens(s string) []string {
	var toks []string
	var cur []rune
	prevLower := false
	flush := func() {
		if len(cur) > 0 {
			toks = append(toks, detectors.Fold(string(cur)))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			if unicode.IsUpper(r) && prevLower {
				flush()
			}
			cur = append(cur, r)
			prevLower = unicode.IsLower(r)
		default:
			flush()
			prevLower = false
		}
	}
	flush()
	return toks
}

// idScanFindings checks the name of every file a batch saw: scanned, skipped or unreadable, since a large scan
// is skipped by size and its name is still known.
func idScanFindings(b batch) []domain.Finding {
	var out []domain.Finding
	add := func(rel string) {
		if f, ok := idScanFinding(rel); ok {
			out = append(out, f)
		}
	}
	for _, fr := range b.files {
		if fr.notRead == nil {
			add(fr.rel)
		}
	}
	for _, s := range b.skipped {
		add(s.Path)
	}
	for _, s := range b.unreadable {
		add(s.Path)
	}
	return out
}
