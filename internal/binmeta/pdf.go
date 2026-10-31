package binmeta

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
)

// Limits on PDF text extraction: streams looked at, bytes inflated from one stream and from the whole file, and
// lines returned.
const (
	maxPDFStreams     = 4096
	maxStreamBytes    = 4 << 20
	maxInflatedBytes  = 32 << 20
	maxPDFLines       = 100_000
	maxPDFLineLetters = 4096
)

// PDFText returns the text drawn by a PDF's content streams, one line per text positioning operator. Streams are
// inflated when they are Flate-compressed and read as they are otherwise. Only literal strings are read: text in
// hex strings or in fonts with custom encodings, and text in images, is not extracted. No OCR is done.
func PDFText(data []byte) []string {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil
	}
	var lines []string
	budget := int64(maxInflatedBytes)
	rest := data
	for n := 0; n < maxPDFStreams && budget > 0 && len(lines) < maxPDFLines; n++ {
		k := bytes.Index(rest, []byte("stream"))
		if k < 0 {
			break
		}
		start := k + len("stream")
		if bytes.HasPrefix(rest[start:], []byte("\r\n")) {
			start += 2
		} else if bytes.HasPrefix(rest[start:], []byte("\n")) {
			start++
		} else {
			rest = rest[start:]
			continue
		}
		end := bytes.Index(rest[start:], []byte("endstream"))
		if end < 0 {
			break
		}
		raw := rest[start : start+end]
		rest = rest[start+end+len("endstream"):]
		content := raw
		if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
			content, _ = io.ReadAll(io.LimitReader(zr, min(maxStreamBytes, budget)))
			zr.Close()
		}
		budget -= int64(len(content))
		lines = append(lines, textLines(content)...)
	}
	if len(lines) > maxPDFLines {
		lines = lines[:maxPDFLines]
	}
	return lines
}

// textLines collects the literal strings of a content stream. Strings shown together stay on one line; a text
// positioning operator (Td, TD, T*, Tm, ', ") or the end of a text object (ET) starts a new one.
func textLines(content []byte) []string {
	var out []string
	var line strings.Builder
	flush := func() {
		if line.Len() > 0 {
			out = append(out, line.String())
			line.Reset()
		}
	}
	for i := 0; i < len(content); i++ {
		c := content[i]
		switch {
		case c == '(':
			s, next := literal(content, i)
			if line.Len()+len(s) <= maxPDFLineLetters {
				line.WriteString(s)
			}
			i = next
		case c == '\'' || c == '"':
			flush()
		case c == 'T' && i+1 < len(content) && strings.IndexByte("dD*m", content[i+1]) >= 0 && delimited(content, i, 2):
			flush()
			i++
		case c == 'E' && i+1 < len(content) && content[i+1] == 'T' && delimited(content, i, 2):
			flush()
			i++
		}
	}
	flush()
	return out
}

// delimited reports whether the operator of length n at i stands alone, between whitespace or delimiters.
func delimited(b []byte, i, n int) bool {
	sep := func(c byte) bool { return strings.IndexByte(" \t\r\n\f\x00()<>[]{}/%", c) >= 0 }
	return (i == 0 || sep(b[i-1])) && (i+n >= len(b) || sep(b[i+n]))
}

// literal reads the PDF literal string that opens at b[i] (PDF 32000-1, 7.3.4.2) and returns it with the index
// of its closing parenthesis. Balanced parentheses nest; escapes are decoded; octal escapes are kept as bytes.
func literal(b []byte, i int) (string, int) {
	var s []byte
	depth := 0
	for k := i; k < len(b); k++ {
		c := b[k]
		switch c {
		case '(':
			depth++
			if depth == 1 {
				continue
			}
		case ')':
			depth--
			if depth == 0 {
				return string(s), k
			}
		case '\\':
			if k+1 >= len(b) {
				return string(s), k
			}
			k++
			switch e := b[k]; e {
			case 'n':
				s = append(s, '\n')
			case 'r':
				s = append(s, '\r')
			case 't':
				s = append(s, '\t')
			case 'b', 'f':
			case '\r', '\n':
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for j := 0; j < 2 && k+1 < len(b) && b[k+1] >= '0' && b[k+1] <= '7'; j++ {
						k++
						v = v*8 + int(b[k]-'0')
					}
					s = append(s, byte(v))
				} else {
					s = append(s, e)
				}
			}
			continue
		}
		s = append(s, c)
	}
	return string(s), len(b)
}
