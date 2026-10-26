package engine

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/walk"
)

// splitter cuts the lines of a file with columns into segments named by their column. It keeps state across
// lines: headers, quoted values that span lines, the statement being read.
type splitter interface {
	// split returns the segments of one line, or nil to scan the line whole. row is true when the line
	// completed a data row, which counts against the row limit.
	split(line string, number int) (segs []segment, row bool)
	// fields are the column names seen, for the inventory and special categories.
	fields() []domain.Field
}

// splitterFor picks the splitter for a path, or nil for plain text.
func splitterFor(p string) splitter {
	switch kind := walk.DataKind(p); kind {
	case "csv", "tsv":
		return &delimited{path: p, format: kind, tsv: kind == "tsv"}
	case "jsonl":
		return &jsonLines{path: p, seen: map[string]bool{}, folded: map[string]string{}}
	}
	if strings.EqualFold(path.Ext(p), ".sql") {
		return &sqlDump{tables: map[string][]string{}}
	}
	return nil
}

// container names the table a data file stands for: its file name without the extension.
func container(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, path.Ext(base))
}

// cell turns line[start:end] into a segment without surrounding spaces and quotes; ok is false when empty.
func cell(line string, start, end int, column, keywords string) (segment, bool) {
	for start < end && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	for end > start && (line[end-1] == ' ' || line[end-1] == '\t' || line[end-1] == '\r') {
		end--
	}
	if start < end && (line[start] == '"' || line[start] == '\'') {
		start++
	}
	if end > start && (line[end-1] == '"' || line[end-1] == '\'') {
		end--
	}
	if start >= end {
		return segment{}, false
	}
	return segment{start: start, end: end, column: column, keywords: keywords}, true
}

// delimited reads CSV and TSV. The first non-empty line is the header unless it looks like data; the
// delimiter is sniffed from it. Quoted values may hold delimiters and span lines.
type delimited struct {
	path, format string
	tsv          bool
	started      bool
	sep          byte
	names        []string
	keys         []string
	header       []domain.Field
	inQuote      bool
	col          int
}

type span struct{ start, end, col int }

func (d *delimited) fields() []domain.Field { return d.header }

func (d *delimited) split(line string, number int) ([]segment, bool) {
	if !d.inQuote && strings.TrimSpace(line) == "" {
		return nil, false
	}
	if !d.started {
		d.started = true
		d.sep = sniff(line, d.tsv)
		spans := d.cells(line)
		if headerLike(line, spans) {
			d.inQuote, d.col = false, 0
			for _, s := range spans {
				seg, ok := cell(line, s.start, s.end, "", "")
				name := ""
				if ok {
					name = line[seg.start:seg.end]
					d.header = append(d.header, domain.Field{Name: name, Container: container(d.path), Format: d.format,
						Location: domain.Location{Path: d.path, Line: number, Column: utf8.RuneCountInString(line[:seg.start]) + 1}})
				}
				d.names = append(d.names, name)
				d.keys = append(d.keys, detectors.Fold(name))
			}
			return nil, false
		}
		return d.segments(line, spans), !d.inQuote
	}
	return d.segments(line, d.cells(line)), !d.inQuote
}

func (d *delimited) segments(line string, spans []span) []segment {
	out := make([]segment, 0, len(spans))
	for _, s := range spans {
		name, kw := "#"+strconv.Itoa(s.col+1), ""
		if s.col < len(d.names) && d.names[s.col] != "" {
			name, kw = d.names[s.col], d.keys[s.col]
		}
		if seg, ok := cell(line, s.start, s.end, name, kw); ok {
			out = append(out, seg)
		}
	}
	return out
}

// cells splits one physical line, continuing a quoted value from the line before.
func (d *delimited) cells(line string) []span {
	var out []span
	col, start, inQ := 0, 0, d.inQuote
	if inQ {
		col = d.col
	}
	atStart := !inQ
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQ:
			if c == '"' {
				if i+1 < len(line) && line[i+1] == '"' {
					i++
				} else {
					inQ = false
				}
			}
		case c == d.sep:
			out = append(out, span{start, i, col})
			col, start, atStart = col+1, i+1, true
			continue
		case atStart && c == '"':
			inQ = true
		case atStart && c == ' ':
			continue
		}
		atStart = false
	}
	out = append(out, span{start, len(line), col})
	d.inQuote, d.col = inQ, col
	return out
}

// sniff picks the delimiter that occurs most often outside quotes: comma, semicolon, tab or pipe.
func sniff(line string, tsv bool) byte {
	counts := map[byte]int{}
	inQ := false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '"':
			inQ = !inQ
		case !inQ && (c == ',' || c == ';' || c == '\t' || c == '|'):
			counts[c]++
		}
	}
	best := byte(',')
	if tsv {
		best = '\t'
	}
	for _, c := range []byte{',', ';', '\t', '|'} {
		if counts[c] > counts[best] {
			best = c
		}
	}
	return best
}

// headerLike says whether a first line names columns: it has letters and no cell holds five or more digits.
func headerLike(line string, spans []span) bool {
	letters := false
	for _, s := range spans {
		if countDigits(line[s.start:s.end]) >= 5 {
			return false
		}
		letters = letters || strings.IndexFunc(line[s.start:s.end], unicode.IsLetter) >= 0
	}
	return letters
}

// maxKeys caps the distinct key paths remembered per JSON Lines file, for files with generated keys.
const maxKeys = 1000

// jsonLines reads one JSON value per line. Each string or number is a segment named by its key path, such as
// customer.cnp; array items take the name of the array.
type jsonLines struct {
	path   string
	seen   map[string]bool
	folded map[string]string
	keys   []domain.Field
}

func (j *jsonLines) fields() []domain.Field { return j.keys }

type jsonFrame struct {
	object, expectKey bool
	key               string
}

func (j *jsonLines) split(line string, number int) ([]segment, bool) {
	t := strings.TrimSpace(line)
	if t == "" {
		return nil, false
	}
	if t[0] != '{' && t[0] != '[' {
		return nil, true
	}
	var stack []jsonFrame
	var out []segment
	keyPath := func() string {
		parts := make([]string, 0, len(stack))
		for _, f := range stack {
			if f.object && f.key != "" {
				parts = append(parts, f.key)
			}
		}
		return strings.Join(parts, ".")
	}
	emit := func(start, end int) {
		name := keyPath()
		kw, ok := j.folded[name]
		if !ok {
			kw = detectors.Fold(name)
			if len(j.folded) < maxKeys {
				j.folded[name] = kw
			}
		}
		out = append(out, segment{start: start, end: end, column: name, keywords: kw})
	}
	for i := 0; i < len(line); {
		c := line[i]
		top := len(stack) - 1
		switch c {
		case '{', '[':
			stack = append(stack, jsonFrame{object: c == '{', expectKey: c == '{'})
			i++
		case '}', ']':
			if top >= 0 {
				stack = stack[:top]
			}
			i++
		case ',':
			if top >= 0 && stack[top].object {
				stack[top].expectKey = true
			}
			i++
		case '"':
			end := stringEnd(line, i)
			if top >= 0 && stack[top].object && stack[top].expectKey {
				stack[top].key, stack[top].expectKey = line[i+1:end], false
				j.remember(keyPath(), stack[top].key, number, utf8.RuneCountInString(line[:i])+2)
			} else if end > i+1 {
				emit(i+1, end)
			}
			i = end + 1
		case ' ', '\t', '\r', ':':
			i++
		default:
			end := i
			for end < len(line) && !strings.ContainsRune(",}] \t\r", rune(line[end])) {
				end++
			}
			if countDigits(line[i:end]) > 0 {
				emit(i, end)
			}
			i = end
		}
	}
	return out, true
}

func (j *jsonLines) remember(full, key string, line, col int) {
	if j.seen[full] || len(j.seen) >= maxKeys {
		return
	}
	j.seen[full] = true
	parent := strings.TrimSuffix(strings.TrimSuffix(full, key), ".")
	if parent == "" {
		parent = container(j.path)
	}
	j.keys = append(j.keys, domain.Field{Name: key, Container: parent, Format: "jsonl", Location: domain.Location{Path: j.path, Line: line, Column: col}})
}

// stringEnd returns the index of the quote that closes the string opening at i, or len(s).
func stringEnd(s string, i int) int {
	for k := i + 1; k < len(s); k++ {
		switch s[k] {
		case '\\':
			k++
		case '"':
			return k
		}
	}
	return len(s)
}

var (
	sqlInsert = regexp.MustCompile(`(?i)^\s*(?:insert|replace)\s+(?:ignore\s+)?into\s+([^\s(]+)\s*(?:\(([^)]*)\))?\s*values\s*`)
	sqlCopy   = regexp.MustCompile(`(?i)^\s*copy\s+([^\s(]+)\s*\(([^)]*)\)\s+from\s+stdin`)
	sqlCreate = regexp.MustCompile(`(?i)^\s*create\s+(?:temp(?:orary)?\s+)?table\s+(?:if\s+not\s+exists\s+)?([^\s(]+)\s*\(`)
)

// sqlDump names the values of SQL dumps by column: INSERT statements with a column list or after the table's
// CREATE TABLE, single or extended, and PostgreSQL COPY blocks. Other lines are scanned whole.
type sqlDump struct {
	tables   map[string][]string
	folded   map[string]string
	creating string
	create   strings.Builder
	cols     []string
	copyCols []string
	values   bool
	depth    int
	quote    byte
	col      int
	start    int
}

func (s *sqlDump) fields() []domain.Field { return nil }

func (s *sqlDump) split(line string, _ int) ([]segment, bool) {
	switch {
	case s.copyCols != nil:
		if strings.TrimSpace(line) == `\.` {
			s.copyCols = nil
			return nil, false
		}
		return s.copyRow(line), false
	case s.creating != "":
		s.addCreate(line)
		return nil, false
	}
	t := strings.TrimLeft(line, " \t")
	if t == "" || !strings.ContainsRune("iIrRcC", rune(t[0])) {
		if s.values {
			return s.tuples(line, 0), false
		}
		return nil, false
	}
	if m := sqlInsert.FindStringSubmatchIndex(line); m != nil {
		s.cols = nil
		if m[4] >= 0 {
			s.cols = columnList(line[m[4]:m[5]])
		} else {
			s.cols = s.tables[tableName(line[m[2]:m[3]])]
		}
		if s.cols == nil {
			return nil, false
		}
		s.values, s.depth, s.quote, s.col = true, 0, 0, 0
		return s.tuples(line, m[1]), false
	}
	if s.values {
		return s.tuples(line, 0), false
	}
	if m := sqlCopy.FindStringSubmatch(line); m != nil {
		s.copyCols = columnList(m[2])
		return nil, false
	}
	if m := sqlCreate.FindStringSubmatchIndex(line); m != nil {
		s.creating = tableName(line[m[2]:m[3]])
		s.create.Reset()
		s.addCreate(line[m[1]-1:])
	}
	return nil, false
}

// addCreate collects a CREATE TABLE body until its closing parenthesis, then records the column names.
func (s *sqlDump) addCreate(line string) {
	if s.create.Len() < 1<<16 {
		s.create.WriteString(line)
		s.create.WriteByte('\n')
	}
	body := s.create.String()
	depth, quote, itemStart := 0, byte(0), 1
	var cols []string
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')' || c == ',' && depth == 1:
			if depth == 1 {
				if name := sqlColumn(body[itemStart:i]); name != "" {
					cols = append(cols, name)
				}
				itemStart = i + 1
			}
			if c == ')' {
				depth--
				if depth == 0 {
					s.tables[s.creating] = cols
					s.creating = ""
					return
				}
			}
		}
	}
	if s.create.Len() >= 1<<16 {
		s.creating = ""
	}
}

var notSQLColumns = map[string]bool{"primary": true, "foreign": true, "constraint": true, "unique": true, "check": true, "key": true, "index": true, "exclude": true, "like": true, "period": true, "fulltext": true, "spatial": true}

// sqlColumn is the column an item of a CREATE TABLE body declares, or "" for a constraint.
func sqlColumn(item string) string {
	f := strings.Fields(item)
	if len(f) == 0 {
		return ""
	}
	name := unquoteSQL(f[0])
	if notSQLColumns[strings.ToLower(name)] {
		return ""
	}
	return name
}

func columnList(s string) []string {
	var out []string
	for _, c := range strings.Split(s, ",") {
		out = append(out, unquoteSQL(strings.TrimSpace(c)))
	}
	return out
}

func tableName(s string) string {
	parts := strings.Split(s, ".")
	return strings.ToLower(unquoteSQL(parts[len(parts)-1]))
}

func unquoteSQL(s string) string { return strings.Trim(s, "`\"[]") }

func (s *sqlDump) keywords(name string) string {
	if s.folded == nil {
		s.folded = map[string]string{}
	}
	kw, ok := s.folded[name]
	if !ok {
		kw = detectors.Fold(name)
		s.folded[name] = kw
	}
	return kw
}

func (s *sqlDump) named(cols []string, i int) (string, string) {
	if i < len(cols) {
		return cols[i], s.keywords(cols[i])
	}
	return "#" + strconv.Itoa(i+1), ""
}

func (s *sqlDump) copyRow(line string) []segment {
	var out []segment
	start, col := 0, 0
	for i := 0; i <= len(line); i++ {
		if i < len(line) && line[i] != '\t' {
			continue
		}
		if v := line[start:i]; v != `\N` {
			name, kw := s.named(s.copyCols, col)
			if seg, ok := cell(line, start, i, name, kw); ok {
				out = append(out, seg)
			}
		}
		start, col = i+1, col+1
	}
	return out
}

// tuples reads VALUES (...), (...) from line[from:], carrying open strings and tuples to the next line.
func (s *sqlDump) tuples(line string, from int) []segment {
	var out []segment
	emit := func(end int) {
		v := strings.TrimSpace(line[s.start:end])
		if strings.EqualFold(v, "null") {
			return
		}
		name, kw := s.named(s.cols, s.col)
		if seg, ok := cell(line, s.start, end, name, kw); ok {
			out = append(out, seg)
		}
	}
	for i := from; i < len(line); i++ {
		c := line[i]
		if s.quote != 0 {
			switch {
			case c == '\\':
				i++
			case c == s.quote && i+1 < len(line) && line[i+1] == c:
				i++
			case c == s.quote:
				s.quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			s.quote = c
		case '(':
			s.depth++
			if s.depth == 1 {
				s.col, s.start = 0, i+1
			}
		case ')':
			if s.depth == 1 {
				emit(i)
			}
			if s.depth > 0 {
				s.depth--
			}
		case ',':
			if s.depth == 1 {
				emit(i)
				s.col, s.start = s.col+1, i+1
			}
		case ';':
			if s.depth == 0 {
				s.values = false
				return out
			}
		}
	}
	if s.depth >= 1 {
		emit(len(line))
	}
	s.start = 0
	return out
}
