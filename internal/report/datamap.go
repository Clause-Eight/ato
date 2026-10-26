package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/datamap"
	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// MapSchemaVersion is the version of the JSON data map, versioned like the findings report.
const MapSchemaVersion = "1.0"

// MapFormats are the formats `pii-eu map` writes.
var MapFormats = []string{"json", "markdown", "html"}

// DataMap is a data map with the tool that made it.
type DataMap struct {
	Tool    string
	Version string
	Map     datamap.Map
}

type jsonMap struct {
	SchemaVersion string          `json:"schema_version"`
	Tool          jsonTool        `json:"tool"`
	Categories    []string        `json:"categories"`
	Components    []jsonComponent `json:"components"`
}

type jsonComponent struct {
	Name       string          `json:"name"`
	Stores     []jsonStore     `json:"stores"`
	Sinks      []jsonSink      `json:"sinks"`
	Recipients []jsonRecipient `json:"recipients"`
}

type jsonStore struct {
	Name       string        `json:"name"`
	Categories []string      `json:"categories"`
	Special    []string      `json:"special_categories"`
	Fields     []jsonField   `json:"fields"`
	Erasure    *[]jsonRemove `json:"erasure,omitempty"`
	Retention  *[]jsonRemove `json:"retention,omitempty"`
}

type jsonRemove struct {
	How  string `json:"how"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

type jsonField struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Special  string `json:"special_category,omitempty"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
}

type jsonSink struct {
	Label    string `json:"label"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
}

type jsonRecipient struct {
	Name     string      `json:"name"`
	Purpose  string      `json:"purpose"`
	EEA      bool        `json:"established_in_eea"`
	Evidence []jsonPlace `json:"evidence"`
}

type jsonPlace struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// WriteMap writes a data map as JSON, as a Markdown draft of a record of processing, or as an HTML page.
func WriteMap(format string, w io.Writer, m DataMap) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(toJSONMap(m))
	case "markdown":
		_, err := io.WriteString(w, markdownMap(m))
		return err
	case "html":
		return writeMapHTML(w, m)
	}
	return fmt.Errorf("unknown map format %q: use json, markdown or html", format)
}

// ReadMap reads a data map written by WriteMap as JSON, for comparing two maps.
func ReadMap(r io.Reader) (datamap.Map, error) {
	var jm jsonMap
	if err := json.NewDecoder(r).Decode(&jm); err != nil {
		return datamap.Map{}, fmt.Errorf("not a pii-eu data map: %w", err)
	}
	if !strings.HasPrefix(jm.SchemaVersion, "1.") {
		return datamap.Map{}, fmt.Errorf("data map schema version %q is not supported", jm.SchemaVersion)
	}
	var m datamap.Map
	for _, c := range jm.Components {
		comp := datamap.Component{Name: c.Name}
		for _, s := range c.Stores {
			store := datamap.Store{Name: s.Name, Categories: s.Categories, Special: s.Special, Assessed: s.Erasure != nil || s.Retention != nil}
			store.Erasure, store.Retention = fromJSONRemovals(s.Erasure), fromJSONRemovals(s.Retention)
			for _, f := range s.Fields {
				store.Fields = append(store.Fields, datamap.Field{Name: f.Name, Category: f.Category, Special: f.Special, Location: domain.Location{Path: f.Path, Line: f.Line}})
			}
			comp.Stores = append(comp.Stores, store)
		}
		for _, s := range c.Sinks {
			comp.Sinks = append(comp.Sinks, datamap.Sink{Label: s.Label, Category: s.Category, Location: domain.Location{Path: s.Path, Line: s.Line}})
		}
		for _, r := range c.Recipients {
			rec := datamap.Recipient{Name: r.Name, Purpose: r.Purpose, EEA: r.EEA}
			for _, e := range r.Evidence {
				rec.Evidence = append(rec.Evidence, domain.Location{Path: e.Path, Line: e.Line})
			}
			comp.Recipients = append(comp.Recipients, rec)
		}
		m.Components = append(m.Components, comp)
	}
	return m, nil
}

func toJSONMap(m DataMap) jsonMap {
	out := jsonMap{SchemaVersion: MapSchemaVersion, Tool: jsonTool{Name: m.Tool, Version: m.Version}, Categories: strs(m.Map.Categories()), Components: []jsonComponent{}}
	for _, c := range m.Map.Components {
		jc := jsonComponent{Name: c.Name, Stores: []jsonStore{}, Sinks: []jsonSink{}, Recipients: []jsonRecipient{}}
		for _, s := range c.Stores {
			js := jsonStore{Name: s.Name, Categories: strs(s.Categories), Special: strs(s.Special), Fields: []jsonField{}}
			for _, f := range s.Fields {
				js.Fields = append(js.Fields, jsonField{Name: f.Name, Category: f.Category, Special: f.Special, Path: f.Location.Path, Line: f.Location.Line})
			}
			if s.Assessed {
				js.Erasure, js.Retention = toJSONRemovals(s.Erasure), toJSONRemovals(s.Retention)
			}
			jc.Stores = append(jc.Stores, js)
		}
		for _, s := range c.Sinks {
			jc.Sinks = append(jc.Sinks, jsonSink{Label: s.Label, Category: s.Category, Path: s.Location.Path, Line: s.Location.Line})
		}
		for _, r := range c.Recipients {
			jr := jsonRecipient{Name: r.Name, Purpose: r.Purpose, EEA: r.EEA, Evidence: []jsonPlace{}}
			for _, e := range r.Evidence {
				jr.Evidence = append(jr.Evidence, jsonPlace{Path: e.Path, Line: e.Line})
			}
			jc.Recipients = append(jc.Recipients, jr)
		}
		out.Components = append(out.Components, jc)
	}
	return out
}

func toJSONRemovals(rs []domain.Removal) *[]jsonRemove {
	out := make([]jsonRemove, 0, len(rs))
	for _, r := range rs {
		out = append(out, jsonRemove{How: r.How, Path: r.Location.Path, Line: r.Location.Line})
	}
	return &out
}

func fromJSONRemovals(rs *[]jsonRemove) []domain.Removal {
	if rs == nil {
		return nil
	}
	out := make([]domain.Removal, 0, len(*rs))
	for _, r := range *rs {
		out = append(out, domain.Removal{How: r.How, Location: domain.Location{Path: r.Path, Line: r.Line}})
	}
	return out
}

// removalText says how a store's data is removed, for people: the first mechanism and where it is.
func removalText(assessed bool, rs []domain.Removal) string {
	switch {
	case !assessed:
		return "–"
	case len(rs) == 0:
		return "none found"
	}
	text := fmt.Sprintf("%s (`%s:%d`)", rs[0].How, rs[0].Location.Path, rs[0].Location.Line)
	if len(rs) > 1 {
		text += fmt.Sprintf(" and %d more", len(rs)-1)
	}
	return text
}

func strs(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func eeaText(eea bool) string {
	if eea {
		return "yes"
	}
	return "no: check transfers and hosting region"
}

func markdownMap(m DataMap) string {
	var b strings.Builder
	b.WriteString("# Data map\n\n")
	fmt.Fprintf(&b, "Drafted by %s %s from the code. It is a starting point for the record of processing activities (GDPR Art. 30): review purposes, legal bases and retention periods, which code cannot show. Erasure and retention show whether the code has a path that removes the data, not that it runs.\n\n", m.Tool, m.Version)
	if len(m.Map.Components) == 0 {
		b.WriteString("No personal data, logging of personal data or third-party recipients found.\n")
		return b.String()
	}
	if cats := m.Map.Categories(); len(cats) > 0 {
		fmt.Fprintf(&b, "Categories of personal data: %s.\n\n", strings.Join(cats, ", "))
	}
	for _, c := range m.Map.Components {
		fmt.Fprintf(&b, "## %s\n\n", c.Name)
		if len(c.Stores) > 0 {
			b.WriteString("| Store | Categories of personal data | Special categories (Art. 9) | Erasure (Art. 17) | Retention (Art. 5(1)(e)) | Source |\n|---|---|---|---|---|---|\n")
			for _, s := range c.Stores {
				src := ""
				if len(s.Fields) > 0 {
					src = fmt.Sprintf("`%s:%d`", s.Fields[0].Location.Path, s.Fields[0].Location.Line)
				}
				fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n", s.Name, dash(strings.Join(s.Categories, ", ")), dash(strings.Join(s.Special, ", ")), removalText(s.Assessed, s.Erasure), removalText(s.Assessed, s.Retention), src)
			}
			b.WriteString("\n")
		}
		if len(c.Recipients) > 0 {
			b.WriteString("| Recipient | Purpose | Established in the EEA | Where |\n|---|---|---|---|\n")
			for _, r := range c.Recipients {
				fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Name, r.Purpose, eeaText(r.EEA), places(r.Evidence, 3))
			}
			b.WriteString("\n")
		}
		if len(c.Sinks) > 0 {
			b.WriteString("Personal data written to logs or telemetry:\n\n")
			for _, s := range c.Sinks {
				fmt.Fprintf(&b, "- %s (%s) at `%s:%d`\n", s.Label, s.Category, s.Location.Path, s.Location.Line)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func dash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

func places(ls []domain.Location, max int) string {
	var parts []string
	for i, l := range ls {
		if i == max {
			parts = append(parts, fmt.Sprintf("and %d more", len(ls)-max))
			break
		}
		parts = append(parts, fmt.Sprintf("`%s:%d`", l.Path, l.Line))
	}
	return strings.Join(parts, ", ")
}
