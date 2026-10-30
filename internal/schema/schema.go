// Package schema reads the field names out of schemas and models: SQL and Laravel migrations, ORM models and
// Mongoose schemas, OpenAPI and JSON Schema documents, protobuf definitions and dbt models. It never reads
// data values.
package schema

import (
	"bytes"
	"path"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// reader extracts fields from one format. Wants is cheap and decides from the path and the start of the file.
type reader struct {
	format string
	wants  func(name string, data []byte) bool
	read   func(data []byte) []domain.Field
}

var readers []reader

func register(r reader) { readers = append(readers, r) }

// Formats lists the formats that can be read.
func Formats() []string {
	out := make([]string, 0, len(readers))
	for _, r := range readers {
		out = append(out, r.format)
	}
	sort.Strings(out)
	return out
}

// Candidate reports whether a file may hold a schema, from its extension alone.
func Candidate(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".sql", ".py", ".rb", ".go", ".java", ".kt", ".ts", ".js", ".mjs", ".cjs", ".php", ".prisma", ".proto", ".json", ".yaml", ".yml":
		return true
	}
	return false
}

// Fields returns the fields declared in a file, with p set as their path. A file can match several readers.
func Fields(p string, data []byte) []domain.Field {
	name := strings.ToLower(path.Base(p))
	var out []domain.Field
	for _, r := range readers {
		if !r.wants(name, data) {
			continue
		}
		for _, f := range r.read(data) {
			f.Format = r.format
			f.Location.Path = p
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Location.Line < out[j].Location.Line })
	return out
}

// lines splits data into lines without their line ends.
func lines(data []byte) []string {
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
}

func hasExt(name string, exts ...string) bool {
	for _, e := range exts {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

func contains(data []byte, subs ...string) bool {
	for _, s := range subs {
		if bytes.Contains(data, []byte(s)) {
			return true
		}
	}
	return false
}

// field builds a field at a 1-based line and column.
func field(name, container string, line, column int) domain.Field {
	return domain.Field{Name: name, Container: container, Location: domain.Location{Line: line, Column: column}}
}

// unquote strips SQL and JSON quoting from an identifier: "name", `name`, [name].
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '`' && s[len(s)-1] == '`' || s[0] == '[' && s[len(s)-1] == ']' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// indent is the number of leading spaces; a tab counts as two.
func indent(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}
