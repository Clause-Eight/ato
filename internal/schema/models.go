package schema

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Line-based model readers: each finds a container (class, struct, model, message) and the fields inside it.
type lineRule struct {
	container *regexp.Regexp
	field     *regexp.Regexp
	// fieldInBody limits fields to lines after a container line; with a closing brace tracked when braces is true.
	braces bool
}

func init() {
	register(reader{format: "django", wants: func(n string, d []byte) bool { return hasExt(n, ".py") && contains(d, "models.Model", "from django") }, read: rules(lineRule{
		container: regexp.MustCompile(`^class\s+(\w+)\s*\(.*Model.*\)\s*:`),
		field:     regexp.MustCompile(`^\s+(\w+)\s*=\s*models\.\w+Field\b`),
	})})
	register(reader{format: "sqlalchemy", wants: func(n string, d []byte) bool {
		return hasExt(n, ".py") && contains(d, "sqlalchemy", "Column(", "mapped_column(")
	}, read: rules(lineRule{
		container: regexp.MustCompile(`^class\s+(\w+)\s*\(`),
		field:     regexp.MustCompile(`^\s+(\w+)\s*(?::\s*Mapped\[[^\]]*\]\s*)?=\s*(?:\w+\.)?(?:Column|mapped_column)\(|^\s+(\w+)\s*:\s*Mapped\[`),
	})})
	register(reader{format: "activerecord", wants: func(n string, d []byte) bool {
		return hasExt(n, ".rb") && contains(d, "create_table", "add_column", "ActiveRecord")
	}, read: readActiveRecord})
	register(reader{format: "gorm", wants: func(n string, d []byte) bool { return hasExt(n, ".go") && contains(d, "gorm") }, read: rules(lineRule{
		container: regexp.MustCompile(`^type\s+(\w+)\s+struct\s*\{`),
		field:     regexp.MustCompile("^\\s+(\\w+)\\s+[\\w.*\\[\\]]+\\s+`[^`]*gorm:"),
		braces:    true,
	})})
	register(reader{format: "jpa", wants: func(n string, d []byte) bool { return hasExt(n, ".java", ".kt") && contains(d, "@Entity", "@Table") }, read: readJPA})
	register(reader{format: "prisma", wants: func(n string, _ []byte) bool { return hasExt(n, ".prisma") }, read: rules(lineRule{
		container: regexp.MustCompile(`^\s*(?:model|type)\s+(\w+)\s*\{`),
		field:     regexp.MustCompile(`^\s+([A-Za-z_]\w*)\s+[A-Z]\w*[?\[\]]*`),
		braces:    true,
	})})
	register(reader{format: "protobuf", wants: func(n string, _ []byte) bool { return hasExt(n, ".proto") }, read: rules(lineRule{
		container: regexp.MustCompile(`^\s*message\s+(\w+)\s*\{`),
		field:     regexp.MustCompile(`^\s*(?:optional\s+|repeated\s+|required\s+)?(?:map<[^>]+>|[\w.]+)\s+(\w+)\s*=\s*\d+`),
		braces:    true,
	})})
}

func rules(r lineRule) func([]byte) []domain.Field {
	return func(data []byte) []domain.Field {
		var out []domain.Field
		container, depth, inside := "", 0, false
		for i, l := range lines(data) {
			if m := r.container.FindStringSubmatch(l); m != nil {
				container, inside = m[1], true
				depth = strings.Count(l, "{") - strings.Count(l, "}")
				continue
			}
			if !inside {
				continue
			}
			if r.braces {
				depth += strings.Count(l, "{") - strings.Count(l, "}")
				if depth <= 0 {
					inside = false
					continue
				}
			}
			if m := r.field.FindStringSubmatchIndex(l); m != nil {
				for g := 2; g+1 < len(m); g += 2 {
					if m[g] >= 0 {
						out = append(out, field(l[m[g]:m[g+1]], container, i+1, m[g]+1))
						break
					}
				}
			}
		}
		return out
	}
}

var (
	arTable  = regexp.MustCompile(`create_table\s*\(?\s*[:"'](\w+)`)
	arColumn = regexp.MustCompile(`^\s*t\.(?:string|text|integer|bigint|float|decimal|date|datetime|time|timestamp|boolean|binary|json|jsonb|uuid|references|belongs_to|column)\s*\(?\s*[:"'](\w+)`)
	arAdd    = regexp.MustCompile(`add_column\s*\(?\s*[:"'](\w+)["']?\s*,\s*[:"'](\w+)`)
)

func readActiveRecord(data []byte) []domain.Field {
	var out []domain.Field
	table := ""
	for i, l := range lines(data) {
		if m := arTable.FindStringSubmatch(l); m != nil {
			table = m[1]
			continue
		}
		if m := arColumn.FindStringSubmatchIndex(l); m != nil {
			out = append(out, field(l[m[2]:m[3]], table, i+1, m[2]+1))
		}
		if m := arAdd.FindStringSubmatchIndex(l); m != nil {
			out = append(out, field(l[m[4]:m[5]], l[m[2]:m[3]], i+1, m[4]+1))
		}
	}
	return out
}

var (
	jpaClass  = regexp.MustCompile(`^\s*(?:public\s+|data\s+|open\s+|abstract\s+)*class\s+(\w+)`)
	jpaColumn = regexp.MustCompile(`@Column\s*\([^)]*\bname\s*=\s*"(\w+)"`)
	jpaJava   = regexp.MustCompile(`^\s*(?:private|protected|public)\s+(?:final\s+)?[\w<>, ?.\[\]]+\s+(\w+)\s*(?:=[^;]*)?;`)
	jpaKotlin = regexp.MustCompile(`^\s*(?:override\s+)?va[lr]\s+(\w+)\s*:`)
)

func readJPA(data []byte) []domain.Field {
	var out []domain.Field
	class, named := "", false
	for i, l := range lines(data) {
		if m := jpaClass.FindStringSubmatch(l); m != nil {
			class = m[1]
		}
		if m := jpaColumn.FindStringSubmatchIndex(l); m != nil {
			out = append(out, field(l[m[2]:m[3]], class, i+1, m[2]+1))
			named = true
			continue
		}
		for _, re := range []*regexp.Regexp{jpaJava, jpaKotlin} {
			if m := re.FindStringSubmatchIndex(l); m != nil && class != "" {
				if named {
					named = false
					break
				}
				out = append(out, field(l[m[2]:m[3]], class, i+1, m[2]+1))
				break
			}
		}
	}
	return out
}
