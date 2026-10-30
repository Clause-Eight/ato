package schema

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

func init() {
	register(reader{format: "doctrine", wants: func(n string, d []byte) bool {
		return hasExt(n, ".php") && contains(d, "Doctrine\\ORM", "@ORM\\", "#[ORM\\")
	}, read: readDoctrine})
	register(reader{format: "laravel", wants: func(n string, d []byte) bool {
		return hasExt(n, ".php") && contains(d, "Schema::") && contains(d, "Blueprint", "$table->")
	}, read: readLaravel})
}

var (
	phpClass = regexp.MustCompile(`\bclass\s+(\w+)`)
	// doctrineColumn is a column mapping as an attribute (#[ORM\Column], also inside a list of attributes) or
	// as a docblock annotation (@ORM\Column). JoinColumn and the other mappings do not match.
	doctrineColumn = regexp.MustCompile(`(?:#\[\s*|@|,\s*ORM\\)(?:ORM\\|Mapping\\|\\?Doctrine\\ORM\\Mapping\\)?Column\b\s*`)
	doctrineName   = regexp.MustCompile(`\bname\s*[:=]\s*['"](\w+)['"]`)
	phpProperty    = regexp.MustCompile(`(?:public|protected|private|var)\s+(?:readonly\s+)?(?:\??[\w\\|]+\s+)?\$(\w+)`)
)

// readDoctrine reads the properties mapped with a Doctrine column, from attributes or docblock annotations.
// The container is the class; a name in the mapping (name: "e_mail", name="e_mail") is the column name and
// replaces the property.
func readDoctrine(data []byte) []domain.Field {
	text := string(data)
	pos := newPositions(text)
	classes := phpClass.FindAllStringSubmatchIndex(text, -1)
	cols := doctrineColumn.FindAllStringIndex(text, -1)
	var out []domain.Field
	for i, m := range cols {
		end := m[1]
		name, at := "", 0
		if end < len(text) && text[end] == '(' {
			args, start := parenBody(text, end)
			if n := doctrineName.FindStringSubmatchIndex(args); n != nil {
				name, at = args[n[2]:n[3]], start+n[2]
			}
			end = start + len(args) + 1
		}
		limit := len(text)
		if i+1 < len(cols) {
			limit = cols[i+1][0]
		}
		if end > limit {
			continue
		}
		p := phpProperty.FindStringSubmatchIndex(text[end:limit])
		if p == nil {
			continue
		}
		if name == "" {
			name, at = text[end+p[2]:end+p[3]], end+p[2]
		}
		line, col := pos.at(at)
		out = append(out, field(name, lastBefore(text, classes, m[0]), line, col))
	}
	return out
}

var (
	laravelTable     = regexp.MustCompile(`Schema::(?:connection\([^)]*\)\s*->\s*)?(?:create|table)\s*\(\s*['"](\w+)['"]`)
	laravelBlueprint = regexp.MustCompile(`Blueprint\s+\$(\w+)`)
	laravelColumn    = regexp.MustCompile(`\$(\w+)\s*->\s*(\w+)\s*\(\s*['"](\w+)['"](?:\s*,\s*['"](\w+)['"])?`)
)

// notLaravelColumns are Blueprint methods whose first argument is not a new column.
var notLaravelColumns = map[string]bool{
	"index": true, "unique": true, "primary": true, "foreign": true, "fulltext": true, "spatialindex": true, "rawindex": true,
	"rename": true, "engine": true, "charset": true, "collation": true, "comment": true, "after": true, "change": true,
	"dropcolumn": true, "dropcolumns": true, "dropindex": true, "dropunique": true, "dropprimary": true, "dropforeign": true,
	"dropfulltext": true, "dropspatialindex": true, "dropmorphs": true, "dropconstrainedforeignid": true,
	"dropforeignidfor": true, "dropifexists": true, "renameindex": true, "references": true, "on": true, "constrained": true,
	"default": true, "storedas": true, "virtualas": true,
}

// readLaravel reads the columns that Laravel migrations add inside Schema::create and Schema::table. The
// container is the table; renameColumn gives the new name.
func readLaravel(data []byte) []domain.Field {
	vars := map[string]bool{"table": true}
	for _, m := range laravelBlueprint.FindAllStringSubmatch(string(data), -1) {
		vars[m[1]] = true
	}
	var out []domain.Field
	table := ""
	for i, l := range lines(data) {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") {
			continue
		}
		if m := laravelTable.FindStringSubmatch(l); m != nil {
			table = m[1]
		}
		if table == "" {
			continue
		}
		for _, m := range laravelColumn.FindAllStringSubmatchIndex(l, -1) {
			method := strings.ToLower(l[m[4]:m[5]])
			if !vars[l[m[2]:m[3]]] || notLaravelColumns[method] {
				continue
			}
			g := 6
			if method == "renamecolumn" {
				if m[8] < 0 {
					continue
				}
				g = 8
			}
			out = append(out, field(l[m[g]:m[g+1]], table, i+1, m[g]+1))
		}
	}
	return out
}
