package engine

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/binmeta/binmetatest"
	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// author is a synthetic name made of two words from the name list.
const author = "Anna Novak"

func fileRequest() Request {
	return Request{Detectors: detectors.All(), Files: FileDetectors()}
}

func byDetector(fs []domain.Finding, name string) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Detector == name {
			out = append(out, f)
		}
	}
	return out
}

func notRead(s domain.Summary, path string) string {
	for _, k := range s.FilesSkipped {
		if k.Path == path && k.NotRead {
			return k.Reason
		}
	}
	return ""
}

const mysqlDump = `-- MySQL dump 10.13  Distrib 8.0
--
-- Table structure for table customers
CREATE TABLE customers (
  id int NOT NULL,
  email varchar(255),
  phone varchar(32),
  sku varchar(16),
  PRIMARY KEY (id)
);
INSERT INTO customers VALUES (1,'a@example.invalid','+40 700 000 001','X1'),(2,'b@example.invalid',NULL,'X2');
INSERT INTO products (id, title) VALUES (1,'Lamp');
`

func TestDatabaseDumpFindings(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(mysqlDump))
	_ = zw.Close()
	copyDump := "-- PostgreSQL database dump\nCOPY public.patients (id, birth_date, notes) FROM stdin;\n1\t1990-01-01\tx\n2\t1991-02-02\ty\n\\.\n"
	res := scanTree(t, map[string]string{
		"backup/shop.sql":     mysqlDump,
		"backup/shop.sql.gz":  gz.String(),
		"backup/clinic.dump":  copyDump,
		"db/seed.sql":         "INSERT INTO users (email, first_name) VALUES ('admin@example.invalid', 'Admin');\n",
		"backup/archive.pgfc": string(binmetatest.PGDump(`COPY public.members (id, email, iban) FROM stdin;`, `COPY public.items (id, sku) FROM stdin;`)),
	}, fileRequest())
	got := map[string]domain.Finding{}
	for _, f := range byDetector(res.Findings, DumpDetector) {
		got[f.Location.Path] = f
		if f.Kind != domain.KindFile || f.Value != "" || strings.Contains(f.Masked, "example") {
			t.Errorf("%s: %+v", f.Location.Path, f)
		}
	}
	if f := got["backup/shop.sql"]; f.Masked != "customers: email, phone" || f.Confidence != domain.High || f.Location.Line != 11 {
		t.Errorf("mysqldump: %+v", f)
	}
	if f := got["backup/shop.sql.gz!/shop.sql"]; f.Masked != "customers: email, phone" {
		t.Errorf("gzip: %+v", f)
	}
	if f := got["backup/clinic.dump"]; f.Masked != "patients: birth_date" || f.Confidence != domain.High || !strings.Contains(strings.Join(f.Reasons, ";"), "2 rows") {
		t.Errorf("COPY: %+v", f)
	}
	if f := got["db/seed.sql"]; f.Confidence != domain.Low {
		t.Errorf("seed: %+v", f)
	}
	if f := got["backup/archive.pgfc"]; f.Masked != "public.members: email, iban" || f.Confidence != domain.Medium {
		t.Errorf("pg_dump custom: %+v", f)
	}
	if notRead(res.Summary, "backup/archive.pgfc") == "" {
		t.Error("a pg_dump archive's rows were not read, so it must be listed as not read")
	}
	if len(got) != 5 {
		t.Errorf("got %d dump findings", len(got))
	}
}

func TestDatabaseDumpCanBeSwitchedOff(t *testing.T) {
	res := scanTree(t, map[string]string{"backup/shop.sql": mysqlDump}, Request{Detectors: detectors.All()})
	if fs := byDetector(res.Findings, DumpDetector); len(fs) != 0 {
		t.Fatalf("got %+v", fs)
	}
}

func TestDocumentAuthor(t *testing.T) {
	if !detectors.KnownName("Anna") || !detectors.KnownName("NOVAK") {
		t.Fatal("the test name is not in the name list")
	}
	res := scanTree(t, map[string]string{
		"docs/offer.docx":   string(binmetatest.Docx(binmetatest.CoreXML(author, author), "body")),
		"docs/tool.xlsx":    string(binmetatest.Docx(binmetatest.CoreXML("Microsoft Office User", "openpyxl"), "")),
		"docs/minutes.odt":  string(binmetatest.ODT(author, "")),
		"docs/broken.pptx":  "PK\x03\x04 broken",
		"docs/no-meta.docx": string(binmetatest.Zip([2]string{"word/document.xml", "<w/>"})),
	}, fileRequest())
	fs := byDetector(res.Findings, AuthorDetector)
	if len(fs) != 2 {
		t.Fatalf("got %+v", fs)
	}
	for _, f := range fs {
		if f.Masked != "•••• •••••" || f.Confidence != domain.Medium || f.Kind != domain.KindFile || f.Value != "" {
			t.Errorf("%+v", f)
		}
	}
	if fs[1].Location.Path != "docs/offer.docx" || fs[1].Reasons[0] != "a person's name in the document's dc:creator, cp:lastModifiedBy" {
		t.Errorf("%+v", fs[1])
	}
	for _, p := range []string{"docs/offer.docx", "docs/tool.xlsx", "docs/minutes.odt", "docs/broken.pptx", "docs/no-meta.docx"} {
		if notRead(res.Summary, p) == "" {
			t.Errorf("%s: the body was not read, so the document must stay not read", p)
		}
	}
	if res.Summary.FilesScanned != 0 {
		t.Errorf("%d files counted as scanned", res.Summary.FilesScanned)
	}
}

func TestPhotoLocation(t *testing.T) {
	tiff := binmetatest.TIFF(48.1372, 11.5756, false)
	res := scanTree(t, map[string]string{
		"photos/a.jpg":      string(binmetatest.JPEG(tiff)),
		"photos/b.heic":     string(binmetatest.HEIF(binmetatest.TIFF(48.1372, 11.5756, true))),
		"photos/plain.jpg":  string(binmetatest.JPEG(nil)),
		"tests/data/c.tiff": string(tiff),
	}, fileRequest())
	fs := byDetector(res.Findings, PhotoGPSDetector)
	if len(fs) != 3 {
		t.Fatalf("got %+v", fs)
	}
	for _, f := range fs {
		if f.Masked != gpsMask || strings.ContainsAny(f.Masked, "0123456789") || f.Category != domain.CategoryLocation {
			t.Errorf("%+v", f)
		}
	}
	if fs[2].Location.Path != "tests/data/c.tiff" || fs[2].Confidence != domain.Low {
		t.Errorf("test path: %+v", fs[2])
	}
}

// mrzLine is the TD3 lower line of the specimen passport in ICAO Doc 9303, a published sample.
const mrzLine = "L898902C36UTO7408122F1204159ZE184226B<<<<<10"

func TestMRZInPDFText(t *testing.T) {
	res := scanTree(t, map[string]string{
		"scans/form.pdf":  string(binmetatest.PDF([]string{"Holder", mrzLine}, true)),
		"scans/plain.pdf": string(binmetatest.PDF([]string{"no zone here"}, false)),
	}, fileRequest())
	fs := byDetector(res.Findings, "mrz")
	if len(fs) != 1 || fs[0].Kind != domain.KindFile || fs[0].Location.Path != "scans/form.pdf" || strings.Contains(fs[0].Masked, "L898") {
		t.Fatalf("got %+v", fs)
	}
	if notRead(res.Summary, "scans/form.pdf") == "" {
		t.Error("a PDF stays not read")
	}
}

func TestIDDocumentScanByName(t *testing.T) {
	cases := map[string]domain.Confidence{
		"uploads/passport_scan.jpg":        domain.Medium,
		"uploads/CarteIdentite-recto.png":  domain.Medium,
		"uploads/dowód_osobisty.pdf":       domain.Medium,
		"uploads/personalausweis.heic":     domain.Medium,
		"uploads/лична-карта.jpg":          domain.Medium,
		"kyc/passports/front.jpg":          domain.Low,
		"uploads/dni.jpg":                  domain.Low,
		"tests/fixtures/id_card_front.png": domain.Low,
	}
	none := []string{"img/passport.svg", "docs/passportjs-logo.png", "img/30-dni.txt", "img/identity.png", "lib/passport.go"}
	files := map[string]string{}
	for p := range cases {
		files[p] = "x"
	}
	for _, p := range none {
		files[p] = "x"
	}
	res := scanTree(t, files, fileRequest())
	got := map[string]domain.Confidence{}
	for _, f := range byDetector(res.Findings, IDScanDetector) {
		got[f.Location.Path] = f.Confidence
		if !strings.Contains(strings.Join(f.Reasons, ";"), "no OCR") {
			t.Errorf("%s: %q", f.Location.Path, f.Reasons)
		}
	}
	for p, want := range cases {
		if got[p] != want {
			t.Errorf("%s: got %v, want %v", p, got[p], want)
		}
	}
	if len(got) != len(cases) {
		t.Errorf("got %v", got)
	}
}

func TestIDDocumentScanSkippedBySize(t *testing.T) {
	b := batch{skipped: []domain.Skipped{{Path: "kyc/passport.jpg", Reason: "larger than 5242880 bytes", NotRead: true}}}
	if fs := idScanFindings(b); len(fs) != 1 || fs[0].Confidence != domain.Medium {
		t.Fatalf("got %+v", fs)
	}
}
