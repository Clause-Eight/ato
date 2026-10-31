package binmeta

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/binmeta/binmetatest"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func TestImageGPSInEveryContainer(t *testing.T) {
	for _, big := range []bool{false, true} {
		tiff := binmetatest.TIFF(-12.3456, 65.4321, big)
		for name, data := range map[string][]byte{
			"jpeg": binmetatest.JPEG(tiff),
			"tiff": tiff,
			"png":  binmetatest.PNG(tiff),
			"webp": binmetatest.WebP(tiff),
			"heif": binmetatest.HEIF(tiff),
		} {
			g, ok := ImageGPS(data)
			if !ok || !near(g.Lat, -12.3456) || !near(g.Lon, 65.4321) {
				t.Errorf("%s (big endian %v): got %+v %v", name, big, g, ok)
			}
		}
	}
}

func TestImageGPSIgnoresMissingAndNullPositions(t *testing.T) {
	for name, data := range map[string][]byte{
		"no exif":    binmetatest.JPEG(nil),
		"zero":       binmetatest.JPEG(binmetatest.TIFF(0, 0, false)),
		"truncated":  binmetatest.JPEG(binmetatest.TIFF(10.5, 20.5, false))[:60],
		"not image":  []byte("hello"),
		"bad header": binmetatest.HEIF([]byte("XX*\x00")),
	} {
		if g, ok := ImageGPS(data); ok {
			t.Errorf("%s: got %+v", name, g)
		}
	}
}

func TestImageGPSRejectsOutOfRange(t *testing.T) {
	tiff := binmetatest.TIFF(10, 20, false)
	tiff[80] = 200 // latitude degrees
	if g, ok := ImageGPS(tiff); ok {
		t.Errorf("got %+v", g)
	}
}

func TestOfficeAuthors(t *testing.T) {
	doc := binmetatest.Docx(binmetatest.CoreXML("Ada Example", "Ada Example"), "body text")
	got, err := OfficeAuthors(doc)
	if err != nil || len(got) != 1 || got[0].Name != "Ada Example" || got[0].Property != "dc:creator, cp:lastModifiedBy" {
		t.Fatalf("got %+v %v", got, err)
	}
	doc = binmetatest.Docx(binmetatest.CoreXML("Ada  Example", "Bo Sample"), "")
	if got, _ = OfficeAuthors(doc); len(got) != 2 || got[0].Name != "Ada Example" || got[1].Property != "cp:lastModifiedBy" {
		t.Fatalf("got %+v", got)
	}
	got, err = OfficeAuthors(binmetatest.ODT("Ada Example", "Bo Sample"))
	if err != nil || len(got) != 2 || got[0].Property != "meta:initial-creator" || got[1].Property != "dc:creator" {
		t.Fatalf("odt: got %+v %v", got, err)
	}
}

func TestOfficeAuthorsWithoutMetadata(t *testing.T) {
	got, err := OfficeAuthors(binmetatest.Zip([2]string{"word/document.xml", "<w/>"}))
	if err != nil || got != nil {
		t.Fatalf("got %+v %v", got, err)
	}
	if _, err := OfficeAuthors([]byte("PK\x03\x04 not really")); err == nil {
		t.Fatal("a broken zip was read")
	}
}

func TestOfficeAuthorsReadsOnlyABoundedMetadataPart(t *testing.T) {
	huge := binmetatest.CoreXML(strings.Repeat("A", 3<<20), "Bo Sample")
	got, err := OfficeAuthors(binmetatest.Docx(huge, ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range got {
		if len(a.Name) > maxAuthorLen {
			t.Fatalf("an author of %d bytes was kept", len(a.Name))
		}
	}
}

func TestOfficeAuthorsIgnoresEntities(t *testing.T) {
	core := `<?xml version="1.0"?><!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/hostname">]>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>&e;</dc:creator></cp:coreProperties>`
	got, _ := OfficeAuthors(binmetatest.Docx(core, ""))
	for _, a := range got {
		if a.Name != "&e;" {
			t.Fatalf("an external entity was expanded: %d bytes", len(a.Name))
		}
	}
}

func TestPDFText(t *testing.T) {
	for _, compress := range []bool{false, true} {
		got := PDFText(binmetatest.PDF([]string{"first line", "with (parens) and \\ backslash"}, compress))
		if len(got) != 2 || got[0] != "first line" || got[1] != `with (parens) and \ backslash` {
			t.Errorf("compress %v: got %q", compress, got)
		}
	}
	if got := PDFText([]byte("not a pdf (x) Tj")); got != nil {
		t.Errorf("got %q", got)
	}
}

func TestPDFTextJoinsStringsOfOneLine(t *testing.T) {
	got := textLines([]byte(`BT [(AB) -20 (CD)] TJ (EF) Tj 0 -12 Td (\101\102) Tj ET`))
	if len(got) != 2 || got[0] != "ABCDEF" || got[1] != "AB" {
		t.Fatalf("got %q", got)
	}
}

func TestPGDumpTables(t *testing.T) {
	data := binmetatest.PGDump(`COPY public.customers (id, "email", phone) FROM stdin;`, `COPY public.customers (id, "email", phone) FROM stdin;`, `COPY public.items (id, sku) FROM stdin;`)
	got := PGDumpTables(data)
	if len(got) != 2 || got[0].Name != "public.customers" || strings.Join(got[0].Columns, ",") != "id,email,phone" || got[1].Name != "public.items" {
		t.Fatalf("got %+v", got)
	}
	if PGDumpTables([]byte("COPY a (b) FROM stdin;")) != nil {
		t.Fatal("read a file without the pg_dump magic")
	}
}

func FuzzImageGPS(f *testing.F) {
	tiff := binmetatest.TIFF(48.1, 11.5, false)
	for _, seed := range [][]byte{tiff, binmetatest.TIFF(48.1, 11.5, true), binmetatest.JPEG(tiff), binmetatest.PNG(tiff), binmetatest.WebP(tiff), binmetatest.HEIF(tiff)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		g, ok := ImageGPS(data)
		if ok && (math.Abs(g.Lat) > 90 || math.Abs(g.Lon) > 180 || math.IsNaN(g.Lat) || math.IsNaN(g.Lon)) {
			t.Fatalf("out of range: %+v", g)
		}
	})
}

func FuzzOfficeAuthors(f *testing.F) {
	f.Add(binmetatest.Docx(binmetatest.CoreXML("Ada Example", "Bo Sample"), ""))
	f.Add(binmetatest.ODT("Ada Example", "Bo Sample"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, _ := OfficeAuthors(data)
		if len(got) > maxAuthors {
			t.Fatalf("%d authors", len(got))
		}
		for _, a := range got {
			if a.Name == "" || len(a.Name) > maxAuthorLen {
				t.Fatalf("author of %d bytes", len(a.Name))
			}
		}
	})
}

func FuzzMetaAuthors(f *testing.F) {
	f.Add([]byte(binmetatest.CoreXML("Ada Example", "Bo Sample")))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, a := range metaAuthors(data) {
			if a.Name == "" || len(a.Name) > maxAuthorLen {
				t.Fatalf("author of %d bytes", len(a.Name))
			}
		}
	})
}

func FuzzPDFText(f *testing.F) {
	f.Add(binmetatest.PDF([]string{"a line"}, true))
	f.Add(binmetatest.PDF([]string{"(nested) \\ line"}, false))
	f.Fuzz(func(t *testing.T, data []byte) {
		if got := PDFText(data); len(got) > maxPDFLines {
			t.Fatalf("%d lines", len(got))
		}
	})
}

func TestReadersSurviveTruncation(t *testing.T) {
	tiff := binmetatest.TIFF(48.1, 11.5, true)
	for _, data := range [][]byte{binmetatest.JPEG(tiff), binmetatest.Docx(binmetatest.CoreXML("Ada Example", ""), "x"), binmetatest.PDF([]string{"x"}, true)} {
		for n := range len(data) {
			ImageGPS(data[:n])
			_, _ = OfficeAuthors(data[:n])
			PDFText(data[:n])
			PGDumpTables(bytes.Clone(data[:n]))
		}
	}
}
