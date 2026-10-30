package schema

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

func init() {
	register(reader{format: "openapi", wants: wantsAPISchema, read: readPropertyDocument})
	register(reader{format: "dbt", wants: func(n string, d []byte) bool {
		return hasExt(n, ".yml", ".yaml") && contains(d, "models:", "sources:") && contains(d, "columns:")
	}, read: readDBT})
}

// wantsAPISchema accepts OpenAPI, Swagger and JSON Schema documents in JSON or YAML.
func wantsAPISchema(name string, data []byte) bool {
	if !hasExt(name, ".json", ".yaml", ".yml") || !contains(data, "properties") {
		return false
	}
	return contains(data, "openapi", "swagger", "$schema", "\"type\": \"object\"", "\"type\":\"object\"", "type: object")
}

func readPropertyDocument(data []byte) []domain.Field {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		return readJSONProperties(data)
	}
	return readYAMLProperties(data)
}

// readJSONProperties walks a JSON document and returns the keys of every "properties" object. JSON gives no
// positions, so each name is placed at the first line that holds it as a key.
func readJSONProperties(data []byte) []domain.Field {
	var doc any
	if json.Unmarshal(data, &doc) != nil {
		return nil
	}
	text := string(data)
	pos := newPositions(text)
	var out []domain.Field
	var walk func(v any, container string)
	walk = func(v any, container string) {
		switch t := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if props, ok := t[k].(map[string]any); ok && k == "properties" {
					names := make([]string, 0, len(props))
					for name := range props {
						names = append(names, name)
					}
					sort.Strings(names)
					for _, name := range names {
						line, col := 1, 1
						if i := strings.Index(text, `"`+name+`"`); i >= 0 {
							line, col = pos.at(i + 1)
						}
						out = append(out, field(name, container, line, col))
						walk(props[name], name)
					}
					continue
				}
				walk(t[k], k)
			}
		case []any:
			for _, x := range t {
				walk(x, container)
			}
		}
	}
	walk(doc, "")
	return out
}

var yamlKey = regexp.MustCompile(`^(\s*)(?:- )?["']?([^"':#\s][^"':#]*?)["']?\s*:`)

// readYAMLProperties reads the keys directly under every `properties:` key, by indentation.
func readYAMLProperties(data []byte) []domain.Field {
	var out []domain.Field
	ls := lines(data)
	parentAt := func(i int, ind int) string {
		for j := i - 1; j >= 0; j-- {
			if m := yamlKey.FindStringSubmatch(ls[j]); m != nil && indent(ls[j]) < ind && m[2] != "properties" {
				return m[2]
			}
		}
		return ""
	}
	for i, l := range ls {
		m := yamlKey.FindStringSubmatch(l)
		if m == nil || m[2] != "properties" {
			continue
		}
		base := indent(l)
		container := parentAt(i, base)
		child := -1
		for j := i + 1; j < len(ls); j++ {
			if strings.TrimSpace(ls[j]) == "" || strings.HasPrefix(strings.TrimSpace(ls[j]), "#") {
				continue
			}
			ind := indent(ls[j])
			if ind <= base {
				break
			}
			if child < 0 {
				child = ind
			}
			if ind != child {
				continue
			}
			if km := yamlKey.FindStringSubmatchIndex(ls[j]); km != nil {
				out = append(out, field(ls[j][km[4]:km[5]], container, j+1, km[4]+1))
			}
		}
	}
	return out
}

var dbtName = regexp.MustCompile(`^\s*-\s*name:\s*["']?([\w.-]+)`)

// readDBT reads the column names listed under `columns:` in dbt schema files; the container is the model.
func readDBT(data []byte) []domain.Field {
	var out []domain.Field
	ls := lines(data)
	model := ""
	for i, l := range ls {
		m := dbtName.FindStringSubmatchIndex(l)
		if m == nil {
			continue
		}
		name := l[m[2]:m[3]]
		if underColumns(ls, i) {
			out = append(out, field(name, model, i+1, m[2]+1))
		} else {
			model = name
		}
	}
	return out
}

// underColumns reports whether the list item at i belongs to a `columns:` key.
func underColumns(ls []string, i int) bool {
	ind := indent(ls[i])
	for j := i - 1; j >= 0; j-- {
		t := strings.TrimSpace(ls[j])
		if t == "" {
			continue
		}
		if indent(ls[j]) < ind && !strings.HasPrefix(t, "-") {
			return strings.HasPrefix(t, "columns:")
		}
	}
	return false
}
