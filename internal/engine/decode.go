package engine

import (
	"encoding/base64"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
)

// decoded is text after decoding, with at[i] the offset in the original text of decoded byte i (at has one
// extra entry, the original length), and how naming the encoding each decoded byte came from ("" when plain).
type decoded struct {
	text string
	at   []int
	how  []string
}

const (
	urlEncoding    = "URL encoding"
	base64Encoding = "base64"
)

// decodedMatches scans text[start:end] again after decoding percent escapes and whole base64 tokens, and keeps
// the matches that cover encoded text, with their spans mapped back to the original line.
func decodedMatches(ctx *lineContext, seg *segment, start, end int, dets []detectors.Detector, matches []match) []match {
	d, ok := decode(ctx.line[start:end])
	if !ok {
		return matches
	}
	dctx := lineContext{path: ctx.path, testPath: ctx.testPath, number: ctx.number, line: d.text, previous: ctx.previous}
	var dseg *segment
	if seg != nil {
		dseg = &segment{start: 0, end: len(d.text), column: seg.column, keywords: seg.keywords}
	}
	found := len(matches)
	for _, m := range scanSegment(&dctx, dseg, 0, len(d.text), dets, nil) {
		// The value itself or a character next to it was encoded, as in cnp%3D followed by the digits.
		how := encodingIn(d.how[max(m.start-1, 0):min(m.end+1, len(d.how))])
		if how == "" {
			continue
		}
		m.start, m.end = start+d.at[m.start], start+d.at[m.end]
		if sameSpan(matches[:found], m) {
			continue
		}
		m.reasons = append(m.reasons, "found after decoding "+how)
		matches = append(matches, m)
	}
	return matches
}

// sameSpan reports whether the plain scan already found this detector over the same text.
func sameSpan(ms []match, m match) bool {
	for _, k := range ms {
		if k.det.Name == m.det.Name && k.start == m.start && k.end == m.end {
			return true
		}
	}
	return false
}

func encodingIn(how []string) string {
	for _, h := range how {
		if h != "" {
			return h
		}
	}
	return ""
}

// decode undoes percent escapes, then whole base64 tokens. ok is false when nothing was decoded.
func decode(s string) (decoded, bool) {
	p, okP := percentDecode(s)
	b, okB := base64Decode(p.text)
	switch {
	case !okP && !okB:
		return decoded{}, false
	case !okB:
		return p, true
	case !okP:
		return b, true
	}
	// Both stages changed the text: map the second stage's offsets through the first.
	out := decoded{text: b.text, at: make([]int, len(b.at)), how: make([]string, len(b.how))}
	for i, j := range b.at {
		out.at[i] = p.at[j]
	}
	for i := range b.how {
		out.how[i] = b.how[i]
		if out.how[i] == "" {
			out.how[i] = p.how[b.at[i]]
		}
	}
	return out, true
}

// identity is s unchanged, every byte mapped to itself.
func identity(s string) decoded {
	d := decoded{text: s, at: make([]int, len(s)+1), how: make([]string, len(s))}
	for i := range d.at {
		d.at[i] = i
	}
	return d
}

// percentDecode replaces every %XX escape by its byte. '+' is left alone: outside form bodies it is a plus.
func percentDecode(s string) (decoded, bool) {
	i := strings.IndexByte(s, '%')
	for i >= 0 && !escapeAt(s, i) {
		next := strings.IndexByte(s[i+1:], '%')
		if next < 0 {
			i = -1
			break
		}
		i += 1 + next
	}
	if i < 0 {
		return identity(s), false
	}
	var b strings.Builder
	d := decoded{at: make([]int, 0, len(s)+1), how: make([]string, 0, len(s))}
	for j := 0; j < len(s); {
		if escapeAt(s, j) {
			b.WriteByte(unhex(s[j+1])<<4 | unhex(s[j+2]))
			d.at, d.how = append(d.at, j), append(d.how, urlEncoding)
			j += 3
			continue
		}
		b.WriteByte(s[j])
		d.at, d.how = append(d.at, j), append(d.how, "")
		j++
	}
	d.text, d.at = b.String(), append(d.at, len(s))
	return d, true
}

func escapeAt(s string, i int) bool {
	return s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2])
}

func isHex(c byte) bool { return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}

// Base64 tokens are decoded only whole, 12 to 64 characters long, and only when they decode to letters and
// digits: an encoded identifier, not arbitrary bytes.
const minBase64, maxBase64 = 12, 64

func isBase64(c byte) bool {
	return isAlnum(rune(c)) || c == '+' || c == '/' || c == '-' || c == '_'
}

func base64Decode(s string) (decoded, bool) {
	var d decoded
	var b strings.Builder
	changed := false
	last := 0
	for i := 0; i < len(s); {
		if !isBase64(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isBase64(s[j]) {
			j++
		}
		k := j
		for k < len(s) && k-j < 2 && s[k] == '=' {
			k++
		}
		if v, ok := base64Token(s[i:k]); ok {
			if !changed {
				d = decoded{at: make([]int, 0, len(s)+1), how: make([]string, 0, len(s))}
				changed = true
			}
			for p := last; p < i; p++ {
				b.WriteByte(s[p])
				d.at, d.how = append(d.at, p), append(d.how, "")
			}
			b.WriteString(v)
			for range v {
				d.at, d.how = append(d.at, i), append(d.how, base64Encoding)
			}
			last = k
		}
		i = k
	}
	if !changed {
		return identity(s), false
	}
	for p := last; p < len(s); p++ {
		b.WriteByte(s[p])
		d.at, d.how = append(d.at, p), append(d.how, "")
	}
	// Every decoded byte maps to the start of its token, so the byte after a token maps to the token's end.
	d.text, d.at = b.String(), append(d.at, len(s))
	return d, true
}

// base64Token decodes one whole token in the standard or URL alphabet, padded or not, and accepts it only when
// it decodes to ASCII letters and digits and is not itself a plain number.
func base64Token(tok string) (string, bool) {
	if len(tok) < minBase64 || len(tok) > maxBase64 {
		return "", false
	}
	body := strings.TrimRight(tok, "=")
	std, url := strings.ContainsAny(body, "+/"), strings.ContainsAny(body, "-_")
	if std && url || strings.Trim(body, "0123456789") == "" {
		return "", false
	}
	enc := base64.StdEncoding
	if url {
		enc = base64.URLEncoding
	}
	if len(body) == len(tok) {
		enc = enc.WithPadding(base64.NoPadding)
	}
	out, err := enc.Strict().DecodeString(tok)
	if err != nil || len(out) == 0 {
		return "", false
	}
	for _, c := range out {
		if !isAlnum(rune(c)) {
			return "", false
		}
	}
	return string(out), true
}
