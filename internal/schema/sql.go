package schema

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

func init() {
	register(reader{format: "sql", wants: func(name string, _ []byte) bool { return hasExt(name, ".sql") }, read: readSQL})
}

var (
	createTable = regexp.MustCompile(`(?is)\bcreate\s+(?:or\s+replace\s+)?(?:temp(?:orary)?\s+)?table\s+(?:if\s+not\s+exists\s+)?([\w."\x60\[\]]+)\s*\(`)
	addColumn   = regexp.MustCompile(`(?is)\balter\s+table\s+(?:if\s+exists\s+)?(?:only\s+)?([\w."\x60\[\]]+)\s+add\s+(?:column\s+)?(?:if\s+not\s+exists\s+)?([\w"\x60\[\]]+)`)
	renameCol   = regexp.MustCompile(`(?is)\balter\s+table\s+(?:if\s+exists\s+)?([\w."\x60\[\]]+)\s+rename\s+(?:column\s+)?[\w"\x60\[\]]+\s+to\s+([\w"\x60\[\]]+)`)
)

var notColumns = map[string]bool{"primary": true, "foreign": true, "constraint": true, "unique": true, "check": true, "key": true, "index": true, "exclude": true, "like": true, "period": true, "fulltext": true, "spatial": true}

func readSQL(data []byte) []domain.Field {
	text := string(data)
	pos := newPositions(text)
	var out []domain.Field
	for _, m := range createTable.FindAllStringSubmatchIndex(text, -1) {
		table := lastPart(unquote(text[m[2]:m[3]]))
		body, start := parenBody(text, m[1]-1)
		for _, item := range splitTopLevel(body, start) {
			word := firstWord(item.text)
			if word == "" || notColumns[strings.ToLower(word)] {
				continue
			}
			line, col := pos.at(item.offset + strings.Index(item.text, word))
			out = append(out, field(unquote(word), table, line, col))
		}
	}
	for _, re := range []*regexp.Regexp{addColumn, renameCol} {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			name := unquote(text[m[4]:m[5]])
			if notColumns[strings.ToLower(name)] {
				continue
			}
			line, col := pos.at(m[4])
			out = append(out, field(name, lastPart(unquote(text[m[2]:m[3]])), line, col))
		}
	}
	return out
}

type item struct {
	text   string
	offset int
}

// parenBody returns the text between the parenthesis at open and its match, and the offset where it starts.
func parenBody(text string, open int) (string, int) {
	depth := 0
	inQuote := byte(0)
	for i := open; i < len(text); i++ {
		c := text[i]
		switch {
		case inQuote != 0:
			if c == inQuote {
				inQuote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			inQuote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return text[open+1 : i], open + 1
			}
		}
	}
	return text[open+1:], open + 1
}

// splitTopLevel splits a column list at commas outside parentheses and quotes, dropping -- comments.
func splitTopLevel(body string, base int) []item {
	var out []item
	depth, start := 0, 0
	inQuote := byte(0)
	for i := 0; i <= len(body); i++ {
		if i == len(body) || inQuote == 0 && depth == 0 && body[i] == ',' {
			out = append(out, item{text: stripComments(body[start:i]), offset: base + start})
			start = i + 1
			continue
		}
		c := body[i]
		switch {
		case inQuote != 0:
			if c == inQuote {
				inQuote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			inQuote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		}
	}
	return out
}

func stripComments(s string) string {
	var b strings.Builder
	for _, l := range strings.Split(s, "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i] + strings.Repeat(" ", len(l)-i)
		}
		b.WriteString(l + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

var identifier = regexp.MustCompile("^\\s*(\"[^\"]+\"|`[^`]+`|\\[[^\\]]+\\]|[A-Za-z_][\\w$]*)")

func firstWord(s string) string {
	m := identifier.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

func lastPart(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return unquote(name[i+1:])
	}
	return name
}

// positions converts byte offsets to 1-based lines and columns.
type positions struct{ starts []int }

func newPositions(text string) positions {
	p := positions{starts: []int{0}}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			p.starts = append(p.starts, i+1)
		}
	}
	return p
}

func (p positions) at(offset int) (int, int) {
	lo, hi := 0, len(p.starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if p.starts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1, offset - p.starts[lo] + 1
}
