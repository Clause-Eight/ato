// Package binmeta reads the few pieces of metadata ATO needs from binary files: GPS coordinates in image EXIF,
// author names in office documents, text in PDF content streams and the table of contents of pg_dump archives.
// Every reader works on bytes already read under a size limit, treats them as hostile (ADR 13) and fails
// closed: malformed input gives no result, never a panic or an unbounded allocation. It imports nothing from
// this module.
package binmeta

import (
	"bytes"
	"encoding/binary"
	"math"
)

// Limits on EXIF parsing, far above what cameras write.
const (
	maxIFDEntries = 512
	maxIFDs       = 8
)

// GPS is a position read from EXIF. Values are decimal degrees, south and west negative.
type GPS struct {
	Lat, Lon float64
}

// ImageGPS returns the GPS position in an image's EXIF metadata. JPEG and TIFF are read by their structure; PNG
// eXIf and WebP EXIF chunks by theirs; HEIF and AVIF by finding the Exif item's TIFF header, since their box
// structure is not walked. ok is false when there is no usable position: missing, out of range, or (0, 0).
func ImageGPS(data []byte) (GPS, bool) {
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xd8}):
		return gpsFromTIFF(jpegExif(data))
	case bytes.HasPrefix(data, []byte("II*\x00")), bytes.HasPrefix(data, []byte("MM\x00*")):
		return gpsFromTIFF(data)
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return gpsFromTIFF(pngExif(data))
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP":
		return gpsFromTIFF(webpExif(data))
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		return gpsFromTIFF(markedExif(data))
	}
	return GPS{}, false
}

// jpegExif returns the TIFF data of the first APP1 Exif segment, walking the markers up to the start of scan.
func jpegExif(data []byte) []byte {
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xff {
			return nil
		}
		marker := data[i+1]
		if marker == 0xff {
			i++
			continue
		}
		if marker == 0xd8 || marker >= 0xd0 && marker <= 0xd7 || marker == 0x01 {
			i += 2
			continue
		}
		if marker == 0xda || marker == 0xd9 {
			return nil
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if size < 2 || i+2+size > len(data) {
			return nil
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xe1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return seg[6:]
		}
		i += 2 + size
	}
	return nil
}

// pngExif returns the data of the eXIf chunk.
func pngExif(data []byte) []byte {
	i := 8
	for i+12 <= len(data) {
		n := int64(binary.BigEndian.Uint32(data[i:]))
		typ := string(data[i+4 : i+8])
		if n > int64(len(data)-i-12) {
			return nil
		}
		if typ == "eXIf" {
			return data[i+8 : i+8+int(n)]
		}
		if typ == "IEND" {
			return nil
		}
		i += 12 + int(n)
	}
	return nil
}

// webpExif returns the data of the EXIF chunk of a RIFF WebP file.
func webpExif(data []byte) []byte {
	i := 12
	for i+8 <= len(data) {
		n := int64(binary.LittleEndian.Uint32(data[i+4:]))
		if n > int64(len(data)-i-8) {
			return nil
		}
		if string(data[i:i+4]) == "EXIF" {
			return bytes.TrimPrefix(data[i+8:i+8+int(n)], []byte("Exif\x00\x00"))
		}
		i += 8 + int(n) + int(n%2)
	}
	return nil
}

// markedExif finds "Exif\0\0" followed by a TIFF header, as HEIF stores its Exif item.
func markedExif(data []byte) []byte {
	for off := 0; ; {
		k := bytes.Index(data[off:], []byte("Exif\x00\x00"))
		if k < 0 {
			return nil
		}
		tiff := data[off+k+6:]
		if bytes.HasPrefix(tiff, []byte("II*\x00")) || bytes.HasPrefix(tiff, []byte("MM\x00*")) {
			return tiff
		}
		off += k + 6
	}
}

// tiffReader reads values from a TIFF structure in its byte order; every read is bounds-checked.
type tiffReader struct {
	data  []byte
	order binary.ByteOrder
}

func (t tiffReader) u16(off int64) (int64, bool) {
	if off < 0 || off+2 > int64(len(t.data)) {
		return 0, false
	}
	return int64(t.order.Uint16(t.data[off:])), true
}

func (t tiffReader) u32(off int64) (int64, bool) {
	if off < 0 || off+4 > int64(len(t.data)) {
		return 0, false
	}
	return int64(t.order.Uint32(t.data[off:])), true
}

// ifdEntry is one field of an image file directory. valueOff is where the value is: inline in the entry when it
// fits in four bytes, elsewhere in the TIFF data otherwise. Entries whose value would lie outside the data are
// dropped.
type ifdEntry struct {
	typ      int64
	count    int64
	valueOff int64
}

// typeSizes are the byte sizes of the TIFF field types read here: BYTE, ASCII, SHORT, LONG, RATIONAL.
var typeSizes = map[int64]int64{1: 1, 2: 1, 3: 2, 4: 4, 5: 8}

func (t tiffReader) ifd(off int64) map[int64]ifdEntry {
	n, ok := t.u16(off)
	if !ok || n > maxIFDEntries {
		return nil
	}
	out := map[int64]ifdEntry{}
	for i := int64(0); i < n; i++ {
		e := off + 2 + 12*i
		tag, ok1 := t.u16(e)
		typ, ok2 := t.u16(e + 2)
		count, ok3 := t.u32(e + 4)
		if !ok1 || !ok2 || !ok3 {
			return out
		}
		size, known := typeSizes[typ]
		if !known || count > 1<<20 {
			continue
		}
		valueOff := e + 8
		if size*count > 4 {
			valueOff, _ = t.u32(e + 8)
		}
		if valueOff+size*count > int64(len(t.data)) {
			continue
		}
		out[tag] = ifdEntry{typ: typ, count: count, valueOff: valueOff}
	}
	return out
}

// TIFF and EXIF tags and types (TIFF 6.0, EXIF 2.32 section 4.6.6): the GPS IFD pointer in IFD0, then latitude
// and longitude with their N/S and E/W references.
const (
	tagGPSIFD    = 0x8825
	tagLatRef    = 1
	tagLat       = 2
	tagLonRef    = 3
	tagLon       = 4
	typeASCII    = 2
	typeLong     = 4
	typeRational = 5
)

func gpsFromTIFF(data []byte) (GPS, bool) {
	if len(data) < 8 {
		return GPS{}, false
	}
	t := tiffReader{data: data}
	switch string(data[:4]) {
	case "II*\x00":
		t.order = binary.LittleEndian
	case "MM\x00*":
		t.order = binary.BigEndian
	default:
		return GPS{}, false
	}
	gpsOff := int64(-1)
	seen := map[int64]bool{}
	off, _ := t.u32(4)
	for k := 0; off > 0 && k < maxIFDs && !seen[off]; k++ {
		seen[off] = true
		if e, ok := t.ifd(off)[tagGPSIFD]; ok && e.typ == typeLong && e.count == 1 {
			gpsOff, _ = t.u32(e.valueOff)
			break
		}
		n, ok := t.u16(off)
		if !ok {
			break
		}
		off, _ = t.u32(off + 2 + 12*n)
	}
	if gpsOff <= 0 {
		return GPS{}, false
	}
	g := t.ifd(gpsOff)
	lat, ok1 := t.degrees(g, tagLat, tagLatRef, 'S', 90)
	lon, ok2 := t.degrees(g, tagLon, tagLonRef, 'W', 180)
	if !ok1 || !ok2 || lat == 0 && lon == 0 {
		return GPS{}, false
	}
	return GPS{Lat: lat, Lon: lon}, true
}

// degrees reads three RATIONALs (degrees, minutes, seconds) and applies the reference letter.
func (t tiffReader) degrees(g map[int64]ifdEntry, tag, refTag int64, negative byte, limit float64) (float64, bool) {
	v, ok := g[tag]
	if !ok || v.typ != typeRational || v.count != 3 {
		return 0, false
	}
	var parts [3]float64
	for i := range int64(3) {
		num, _ := t.u32(v.valueOff + 8*i)
		den, _ := t.u32(v.valueOff + 8*i + 4)
		if den == 0 {
			return 0, false
		}
		parts[i] = float64(num) / float64(den)
	}
	d := parts[0] + parts[1]/60 + parts[2]/3600
	if math.IsNaN(d) || d > limit {
		return 0, false
	}
	if ref, ok := g[refTag]; ok && ref.typ == typeASCII && ref.count >= 1 && t.data[ref.valueOff] == negative {
		d = -d
	}
	return d, true
}
