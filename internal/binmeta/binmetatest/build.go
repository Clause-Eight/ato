// Package binmetatest builds small synthetic binary files for tests: images with EXIF GPS, office documents,
// PDFs and pg_dump archives. Nothing here comes from a real file.
package binmetatest

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// TIFF is a minimal TIFF structure whose IFD0 points to a GPS IFD holding lat and lon.
func TIFF(lat, lon float64, big bool) []byte {
	var order binary.ByteOrder = binary.LittleEndian
	head := []byte("II*\x00")
	if big {
		order, head = binary.BigEndian, []byte("MM\x00*")
	}
	b := &bytes.Buffer{}
	b.Write(head)
	u16 := func(v int) { _ = binary.Write(b, order, uint16(v)) }
	u32 := func(v int) { _ = binary.Write(b, order, uint32(v)) }
	entry := func(tag, typ, count, value int) { u16(tag); u16(typ); u32(count); u32(value) }
	ref := func(c byte) int {
		if order == binary.BigEndian {
			return int(c) << 24
		}
		return int(c)
	}
	u32(8)
	// IFD0 at 8: one entry, then the next-IFD offset.
	u16(1)
	entry(0x8825, 4, 1, 26)
	u32(0)
	// GPS IFD at 26: four entries (2+48+4 bytes), rationals from 80.
	u16(4)
	latRef, lonRef := byte('N'), byte('E')
	if lat < 0 {
		latRef = 'S'
	}
	if lon < 0 {
		lonRef = 'W'
	}
	entry(1, 2, 2, ref(latRef))
	entry(2, 5, 3, 80)
	entry(3, 2, 2, ref(lonRef))
	entry(4, 5, 3, 104)
	u32(0)
	for _, v := range []float64{math.Abs(lat), math.Abs(lon)} {
		d := math.Floor(v)
		m := math.Floor((v - d) * 60)
		s := (v - d - m/60) * 3600
		u32(int(d))
		u32(1)
		u32(int(m))
		u32(1)
		u32(int(math.Round(s * 1000)))
		u32(1000)
	}
	return b.Bytes()
}

// JPEG wraps EXIF TIFF data in a JPEG with a JFIF segment, an APP1 Exif segment and a stub scan.
func JPEG(tiff []byte) []byte {
	b := &bytes.Buffer{}
	b.Write([]byte{0xff, 0xd8})
	b.Write([]byte{0xff, 0xe0, 0, 16})
	b.WriteString("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	if tiff != nil {
		b.Write([]byte{0xff, 0xe1})
		_ = binary.Write(b, binary.BigEndian, uint16(len(tiff)+8))
		b.WriteString("Exif\x00\x00")
		b.Write(tiff)
	}
	b.Write([]byte{0xff, 0xda, 0, 2, 1, 2, 3, 0, 4, 0xff, 0xd9})
	return b.Bytes()
}

// PNG wraps EXIF TIFF data in a PNG eXIf chunk. CRCs are left zero; the reader does not check them.
func PNG(tiff []byte) []byte {
	b := &bytes.Buffer{}
	b.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(typ string, data []byte) {
		_ = binary.Write(b, binary.BigEndian, uint32(len(data)))
		b.WriteString(typ)
		b.Write(data)
		b.Write([]byte{0, 0, 0, 0})
	}
	chunk("IHDR", make([]byte, 13))
	chunk("eXIf", tiff)
	chunk("IDAT", []byte{0, 0})
	chunk("IEND", nil)
	return b.Bytes()
}

// WebP wraps EXIF TIFF data in a RIFF WebP EXIF chunk.
func WebP(tiff []byte) []byte {
	body := &bytes.Buffer{}
	body.WriteString("WEBP")
	chunk := func(typ string, data []byte) {
		body.WriteString(typ)
		_ = binary.Write(body, binary.LittleEndian, uint32(len(data)))
		body.Write(data)
		if len(data)%2 == 1 {
			body.WriteByte(0)
		}
	}
	chunk("VP8 ", []byte{1, 2, 3})
	chunk("EXIF", tiff)
	b := &bytes.Buffer{}
	b.WriteString("RIFF")
	_ = binary.Write(b, binary.LittleEndian, uint32(body.Len()))
	b.Write(body.Bytes())
	return b.Bytes()
}

// HEIF puts EXIF TIFF data after an ftyp box, the way a HEIF Exif item stores it (offset, "Exif\0\0", TIFF).
func HEIF(tiff []byte) []byte {
	b := &bytes.Buffer{}
	b.Write([]byte{0, 0, 0, 24})
	b.WriteString("ftypheic\x00\x00\x00\x00mif1heic")
	b.Write([]byte{0, 0, 0, 8})
	b.WriteString("meta")
	b.Write([]byte{0, 0, 0, 6})
	b.WriteString("Exif\x00\x00")
	b.Write(tiff)
	return b.Bytes()
}

// Zip builds a zip file from names and contents, in the given order.
func Zip(files ...[2]string) []byte {
	b := &bytes.Buffer{}
	w := zip.NewWriter(b)
	for _, f := range files {
		fw, _ := w.Create(f[0])
		_, _ = fw.Write([]byte(f[1]))
	}
	_ = w.Close()
	return b.Bytes()
}

// CoreXML is an Office Open XML core properties part.
func CoreXML(creator, lastModifiedBy string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
<dc:title>Quarterly figures</dc:title><dc:creator>` + creator + `</dc:creator><cp:lastModifiedBy>` + lastModifiedBy + `</cp:lastModifiedBy>
<dcterms:created>2026-01-02T03:04:05Z</dcterms:created></cp:coreProperties>`
}

// Docx is a Word document with the given core properties and a body.
func Docx(core, body string) []byte {
	return Zip([2]string{"[Content_Types].xml", `<Types/>`}, [2]string{"word/document.xml", "<w:document><w:body><w:t>" + body + "</w:t></w:body></w:document>"}, [2]string{"docProps/core.xml", core})
}

// ODT is an OpenDocument text file whose meta.xml names the initial creator and the creator.
func ODT(initial, creator string) []byte {
	meta := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<office:meta><meta:initial-creator>` + initial + `</meta:initial-creator><dc:creator>` + creator + `</dc:creator></office:meta></office:document-meta>`
	return Zip([2]string{"mimetype", "application/vnd.oasis.opendocument.text"}, [2]string{"meta.xml", meta}, [2]string{"content.xml", "<office:document-content/>"})
}

// PDF is a one-page PDF whose content stream shows each line with Tj after a Td move, Flate-compressed when
// compress is set. Object offsets are not exact; the reader does not need them.
func PDF(lines []string, compress bool) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 10 Tf 50 700 Td\n")
	for _, l := range lines {
		l = strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(l)
		fmt.Fprintf(&content, "(%s) Tj 0 -12 Td\n", l)
	}
	content.WriteString("ET\n")
	data := []byte(content.String())
	filter := ""
	if compress {
		z := &bytes.Buffer{}
		w := zlib.NewWriter(z)
		_, _ = w.Write(data)
		_ = w.Close()
		data, filter = z.Bytes(), " /Filter /FlateDecode"
	}
	b := &bytes.Buffer{}
	b.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	b.WriteString("1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n")
	b.WriteString("2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n")
	b.WriteString("3 0 obj << /Type /Page /Parent 2 0 R /Contents 4 0 R >> endobj\n")
	fmt.Fprintf(b, "4 0 obj << /Length %d%s >>\nstream\n", len(data), filter)
	b.Write(data)
	b.WriteString("\nendstream\nendobj\ntrailer << /Root 1 0 R >>\n%%EOF\n")
	return b.Bytes()
}

// PGDump is a stand-in for a pg_dump custom-format archive: the magic, binary header bytes and a table of
// contents holding the given COPY statements between length prefixes, followed by opaque compressed data.
func PGDump(copies ...string) []byte {
	b := &bytes.Buffer{}
	b.WriteString("PGDMP\x01\x10\x00\x04\x08\x01\x01\x00\x00\x00")
	for _, c := range copies {
		_ = binary.Write(b, binary.LittleEndian, uint32(len(c)))
		b.WriteString(c)
		b.WriteByte(0)
	}
	b.Write([]byte{0x78, 0x9c, 0, 1, 2, 3, 0, 0})
	return b.Bytes()
}
