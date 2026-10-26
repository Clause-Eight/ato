package infra

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokKind int

const (
	tEOF tokKind = iota
	tNewline
	tIdent
	tNumber
	tString
	tPunct
)

// token is one lexical unit of HCL native syntax. A quoted string or heredoc is one token whose parts are its
// literal text and the source of each interpolation.
type token struct {
	kind       tokKind
	text       string
	parts      []part
	line, col  int
	start, end int
}

// part is a piece of a string template: literal text, or the source of a ${...} or %{...} sequence.
type part struct {
	lit    string
	src    string
	isExpr bool
	line   int
}

var multiPunct = []string{"...", "==", "!=", "<=", ">=", "&&", "||", "=>"}

// lex splits HCL source into tokens. It never fails: text it does not understand becomes punctuation, so a
// broken file still yields the blocks around the broken part. line is the line number of the first byte.
func lex(src string, line int) []token {
	l := lexer{src: src, line: line, col: 1}
	var out []token
	for {
		t := l.next()
		out = append(out, t)
		if t.kind == tEOF {
			return out
		}
	}
}

type lexer struct {
	src       string
	pos       int
	line, col int
}

func (l *lexer) advance(n int) {
	for i := 0; i < n && l.pos < len(l.src); i++ {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *lexer) rest() string { return l.src[l.pos:] }

func (l *lexer) next() token {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			l.advance(1)
		case c == '#' || strings.HasPrefix(l.rest(), "//"):
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.advance(1)
			}
		case strings.HasPrefix(l.rest(), "/*"):
			end := strings.Index(l.rest()[2:], "*/")
			if end < 0 {
				l.advance(len(l.rest()))
			} else {
				l.advance(end + 4)
			}
		default:
			return l.token()
		}
	}
	return token{kind: tEOF, line: l.line, col: l.col, start: l.pos, end: l.pos}
}

func (l *lexer) token() token {
	t := token{line: l.line, col: l.col, start: l.pos}
	c := l.src[l.pos]
	r, size := utf8.DecodeRuneInString(l.rest())
	switch {
	case c == '\n':
		t.kind = tNewline
		l.advance(1)
	case c == '"':
		t.kind = tString
		l.advance(1)
		t.parts = l.quoted()
	case strings.HasPrefix(l.rest(), "<<") && l.heredocStart():
		t.kind = tString
		t.parts = l.heredoc()
	case c >= '0' && c <= '9':
		t.kind = tNumber
		l.number()
	case r == '_' || unicode.IsLetter(r):
		t.kind = tIdent
		l.advance(size)
		for l.pos < len(l.src) {
			r, size := utf8.DecodeRuneInString(l.rest())
			if r != '_' && r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
			l.advance(size)
		}
	default:
		t.kind = tPunct
		n := size
		for _, p := range multiPunct {
			if strings.HasPrefix(l.rest(), p) {
				n = len(p)
				break
			}
		}
		l.advance(n)
	}
	t.end = l.pos
	t.text = l.src[t.start:t.end]
	return t
}

func (l *lexer) number() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' && l.pos+1 < len(l.src) && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9':
		case (c == 'e' || c == 'E') && l.pos+1 < len(l.src):
			if n := l.src[l.pos+1]; n == '+' || n == '-' {
				l.advance(1)
			}
		default:
			return
		}
		l.advance(1)
	}
}

// quoted reads a quoted string after its opening quote, up to and including the closing quote. A string never
// spans lines, so an unterminated string ends at the line end.
func (l *lexer) quoted() []part {
	var parts []part
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, part{lit: lit.String()})
			lit.Reset()
		}
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '"':
			l.advance(1)
			flush()
			return parts
		case c == '\n':
			flush()
			return parts
		case c == '\\' && l.pos+1 < len(l.src):
			lit.WriteString(unescape(l.src[l.pos+1]))
			l.advance(2)
		case strings.HasPrefix(l.rest(), "$${") || strings.HasPrefix(l.rest(), "%%{"):
			lit.WriteString(l.src[l.pos+1 : l.pos+3])
			l.advance(3)
		case strings.HasPrefix(l.rest(), "${") || strings.HasPrefix(l.rest(), "%{"):
			flush()
			parts = append(parts, l.interpolation())
		default:
			_, size := utf8.DecodeRuneInString(l.rest())
			lit.WriteString(l.src[l.pos : l.pos+size])
			l.advance(size)
		}
	}
	flush()
	return parts
}

func unescape(c byte) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	}
	return string(c)
}

// interpolation reads ${...} or %{...} and returns its inner source. A directive (%{if}, %{for}) is kept as an
// expression that cannot be resolved.
func (l *lexer) interpolation() part {
	directive := l.src[l.pos] == '%'
	line := l.line
	l.advance(2)
	start := l.pos
	end := skipBraces(l.src, l.pos)
	src := strings.Trim(l.src[start:end], "~ \t")
	l.advance(end - l.pos)
	if l.pos < len(l.src) {
		l.advance(1)
	}
	if directive {
		src = "%{" + src + "}"
	}
	return part{src: src, isExpr: true, line: line}
}

// skipBraces returns the offset of the brace that closes an interpolation starting at pos, skipping nested
// braces and quoted strings.
func skipBraces(src string, pos int) int {
	depth := 0
	for i := pos; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		case '"':
			i = skipQuoted(src, i+1)
		case '\n':
			return i
		}
	}
	return len(src)
}

// skipQuoted returns the offset of the quote that closes a string whose content starts at pos.
func skipQuoted(src string, pos int) int {
	for i := pos; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '"', '\n':
			return i
		case '$', '%':
			if i+1 < len(src) && src[i+1] == '{' {
				i = skipBraces(src, i+2)
			}
		}
	}
	return len(src)
}

// heredocStart reports whether << starts a heredoc: <<EOF or <<-EOF followed by the end of the line.
func (l *lexer) heredocStart() bool {
	rest := strings.TrimPrefix(strings.TrimPrefix(l.rest(), "<<"), "-")
	end := strings.IndexByte(rest, '\n')
	if end < 0 {
		return false
	}
	marker := strings.TrimRight(rest[:end], " \t\r")
	return marker != "" && isIdent(marker)
}

// heredoc reads a heredoc up to and including its closing marker line, without the final line end.
func (l *lexer) heredoc() []part {
	l.advance(2)
	if l.src[l.pos] == '-' {
		l.advance(1)
	}
	nl := strings.IndexByte(l.rest(), '\n')
	marker := strings.TrimRight(l.rest()[:nl], " \t\r")
	l.advance(nl + 1)
	var body strings.Builder
	bodyLine := l.line
	for l.pos < len(l.src) {
		end := strings.IndexByte(l.rest(), '\n')
		text := l.rest()
		if end >= 0 {
			text = text[:end]
		}
		if strings.TrimSpace(text) == marker {
			l.advance(len(text))
			break
		}
		body.WriteString(text + "\n")
		if end < 0 {
			l.advance(len(text))
			break
		}
		l.advance(end + 1)
	}
	return templateParts(body.String(), bodyLine)
}

// templateParts splits heredoc text into literal and interpolation parts. Heredocs have no escapes.
func templateParts(s string, line int) []part {
	var parts []part
	for len(s) > 0 {
		i := strings.Index(s, "${")
		if j := strings.Index(s, "%{"); j >= 0 && (i < 0 || j < i) {
			i = j
		}
		if i < 0 {
			parts = append(parts, part{lit: s})
			break
		}
		if i > 0 {
			parts = append(parts, part{lit: s[:i]})
			line += strings.Count(s[:i], "\n")
		}
		end := skipBraces(s, i+2)
		src := strings.Trim(s[i+2:end], "~ \t")
		if s[i] == '%' {
			src = "%{" + src + "}"
		}
		parts = append(parts, part{src: src, isExpr: true, line: line})
		if end >= len(s) {
			break
		}
		s = s[end+1:]
	}
	return parts
}

func isIdent(s string) bool {
	for i, r := range s {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || r != '-' && !unicode.IsDigit(r)) {
			return false
		}
	}
	return s != ""
}
