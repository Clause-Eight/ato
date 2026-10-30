package schema

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

func init() {
	register(reader{format: "typeorm", wants: func(n string, d []byte) bool {
		return hasExt(n, ".ts", ".js", ".mjs", ".cjs") && contains(d, "@Entity", "@ChildEntity") && contains(d, "Column")
	}, read: readTypeORM})
	register(reader{format: "mongoose", wants: func(n string, d []byte) bool {
		return hasExt(n, ".ts", ".js", ".mjs", ".cjs") && contains(d, "Schema(", "Schema<") && contains(d, "mongoose")
	}, read: readMongoose})
}

var (
	jsClass        = regexp.MustCompile(`\bclass\s+([A-Za-z_$][\w$]*)`)
	typeormColumn  = regexp.MustCompile(`@(?:Column|PrimaryColumn|PrimaryGeneratedColumn|CreateDateColumn|UpdateDateColumn|DeleteDateColumn|VersionColumn|ObjectIdColumn)\s*\(`)
	typeormName    = regexp.MustCompile(`[{,]\s*name\s*:\s*['"\x60]([\w$]+)['"\x60]`)
	jsDecorator    = regexp.MustCompile(`^\s*@[\w$.]+\s*`)
	jsMember       = regexp.MustCompile(`^\s*(?:(?:public|private|protected|readonly|declare|override|static|accessor)\s+)*([A-Za-z_$][\w$]*)\s*[!?]?\s*[:;=\n}]`)
	jsBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// readTypeORM reads the properties marked with a TypeORM column decorator. The container is the class; a name
// in the decorator's options (`@Column({ name: "e_mail" })`) is the column name and replaces the property.
func readTypeORM(data []byte) []domain.Field {
	text := blankComments(string(data))
	pos := newPositions(text)
	classes := jsClass.FindAllStringSubmatchIndex(text, -1)
	var out []domain.Field
	for _, m := range typeormColumn.FindAllStringIndex(text, -1) {
		args, start := jsBody(text, m[1]-1)
		after := start + len(args) + 1
		if after > len(text) {
			continue
		}
		name, at := "", 0
		if n := typeormName.FindStringSubmatchIndex(args); n != nil {
			name, at = args[n[2]:n[3]], start+n[2]
		}
		rest := text[after:]
		skipped := 0
		for {
			d := jsDecorator.FindStringIndex(rest)
			if d == nil {
				break
			}
			cut := d[1]
			if cut < len(rest) && rest[cut] == '(' {
				body, _ := jsBody(rest, cut)
				cut += len(body) + 2
				if cut > len(rest) {
					cut = len(rest)
				}
			}
			rest, skipped = rest[cut:], skipped+cut
		}
		p := jsMember.FindStringSubmatchIndex(rest)
		if p == nil {
			continue
		}
		if name == "" {
			name, at = rest[p[2]:p[3]], after+skipped+p[2]
		}
		line, col := pos.at(at)
		out = append(out, field(name, lastBefore(text, classes, m[0]), line, col))
	}
	return out
}

// lastBefore returns the name of the last match (group 1) that starts before offset.
func lastBefore(text string, matches [][]int, offset int) string {
	name := ""
	for _, c := range matches {
		if c[0] >= offset {
			break
		}
		name = text[c[2]:c[3]]
	}
	return name
}

var (
	mongooseSchema = regexp.MustCompile(`(?:\b([A-Za-z_$][\w$]*)\s*=\s*)?new\s+(?:mongoose\.)?Schema\s*(?:<[^>()]*>)?\s*\(\s*\{`)
	mongooseModel  = regexp.MustCompile(`\bmodel\s*(?:<[^>()]*>)?\s*\(\s*['"\x60]([\w$]+)['"\x60]\s*,\s*(?:([A-Za-z_$][\w$]*)\s*[,)]|new\s)`)
)

// readMongoose reads the paths of Mongoose schemas: the keys of the definition object, nested objects as
// dotted paths ("address.city"), and objects inside arrays. The container is the model the schema is
// registered as, or else the variable's name without "Schema".
func readMongoose(data []byte) []domain.Field {
	text := blankComments(string(data))
	pos := newPositions(text)
	models := map[string]string{}
	var inline [][]int
	for _, m := range mongooseModel.FindAllStringSubmatchIndex(text, -1) {
		if m[4] >= 0 {
			models[text[m[4]:m[5]]] = text[m[2]:m[3]]
		} else {
			inline = append(inline, m)
		}
	}
	var out []domain.Field
	for _, m := range mongooseSchema.FindAllStringSubmatchIndex(text, -1) {
		container := ""
		if m[2] >= 0 {
			v := text[m[2]:m[3]]
			container = models[v]
			if container == "" {
				container = strings.TrimSuffix(strings.TrimSuffix(v, "Schema"), "schema")
			}
		}
		for _, im := range inline {
			if im[0] < m[0] && im[1] > m[0] {
				container = text[im[2]:im[3]]
			}
		}
		body, start := jsBody(text, m[1]-1)
		out = append(out, objectPaths(text, pos, body, start, "", container, 0)...)
	}
	return out
}

// objectPaths lists the keys of a JS object literal, descending into nested objects that are not a field's
// options (those have a "type" key) and into objects inside arrays.
func objectPaths(text string, pos positions, body string, base int, prefix, container string, depth int) []domain.Field {
	if depth > 8 {
		return nil
	}
	var out []domain.Field
	for _, it := range splitJS(body, base) {
		key, value, at := jsKey(it.text)
		if key == "" {
			continue
		}
		line, col := pos.at(it.offset + at)
		out = append(out, field(prefix+key, container, line, col))
		v := strings.TrimSpace(value)
		voff := it.offset + len(it.text) - len(strings.TrimLeft(value, " \t\r\n"))
		if strings.HasPrefix(v, "[") {
			inner, s := jsBody(text, voff)
			v = strings.TrimSpace(inner)
			if !strings.HasPrefix(v, "{") {
				continue
			}
			voff = s + strings.Index(inner, v)
		}
		if strings.HasPrefix(v, "{") {
			inner, s := jsBody(text, voff)
			if hasKey(inner, s, "type") {
				continue
			}
			out = append(out, objectPaths(text, pos, inner, s, prefix+key+".", container, depth+1)...)
		}
	}
	return out
}

func hasKey(body string, base int, name string) bool {
	for _, it := range splitJS(body, base) {
		if k, _, _ := jsKey(it.text); k == name {
			return true
		}
	}
	return false
}

var jsKeyRe = regexp.MustCompile(`^\s*(?:['"]([\w$.-]+)['"]|([A-Za-z_$][\w$]*))\s*:`)

// jsKey splits an object member `key: value` and returns the key's offset in the member. Spreads, shorthand
// members and methods give no key.
func jsKey(s string) (string, string, int) {
	m := jsKeyRe.FindStringSubmatchIndex(s)
	if m == nil {
		return "", "", 0
	}
	g := 2
	if m[2] < 0 {
		g = 4
	}
	return s[m[g]:m[g+1]], s[m[1]:], m[g]
}

// jsBody returns the text between the bracket at open ((, [ or {) and its match, and the offset where it
// starts. Strings are skipped; an unclosed bracket runs to the end of the text.
func jsBody(text string, open int) (string, int) {
	depth := 0
	quote := byte(0)
	for i := open; i < len(text); i++ {
		c := text[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				return text[open+1 : i], open + 1
			}
		}
	}
	if open+1 > len(text) {
		return "", len(text)
	}
	return text[open+1:], open + 1
}

// splitJS splits the members of an object or array literal at top-level commas.
func splitJS(body string, base int) []item {
	var out []item
	depth, start := 0, 0
	quote := byte(0)
	for i := 0; i <= len(body); i++ {
		if i == len(body) || quote == 0 && depth == 0 && body[i] == ',' {
			if strings.TrimSpace(body[start:i]) != "" {
				out = append(out, item{text: body[start:i], offset: base + start})
			}
			start = i + 1
			continue
		}
		c := body[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		}
	}
	return out
}

// blankComments replaces // and /* */ comments with spaces, keeping offsets and line ends, so commented-out
// code is not read. A // inside a string on the same line is left alone when a quote opens before it.
func blankComments(text string) string {
	text = jsBlockComment.ReplaceAllStringFunc(text, func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, s)
	})
	ls := strings.Split(text, "\n")
	for i, l := range ls {
		if j := lineComment(l); j >= 0 {
			ls[i] = l[:j] + strings.Repeat(" ", len(l)-j)
		}
	}
	return strings.Join(ls, "\n")
}

// lineComment returns where a // comment starts on a line outside strings, or -1. A // right after a colon is
// taken for a URL.
func lineComment(l string) int {
	quote := byte(0)
	for i := 0; i < len(l); i++ {
		c := l[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '/' && i+1 < len(l) && l[i+1] == '/' && (i == 0 || l[i-1] != ':'):
			return i
		}
	}
	return -1
}
