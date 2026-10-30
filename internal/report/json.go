package report

import (
	"encoding/json"
	"io"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// SchemaVersion is the version of the JSON report. A new optional field is a minor bump; a removed or changed field is a major bump.
const SchemaVersion = "1.3"

func init() { register("json", writeJSON) }

type jsonReport struct {
	SchemaVersion string        `json:"schema_version"`
	Tool          jsonTool      `json:"tool"`
	Summary       jsonSummary   `json:"summary"`
	Findings      []jsonFinding `json:"findings"`
}

type jsonTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type jsonSummary struct {
	Findings     int           `json:"findings"`
	FilesScanned int           `json:"files_scanned"`
	FilesSkipped []jsonSkipped `json:"files_skipped"`
	Unreadable   []jsonSkipped `json:"unreadable"`
	Ignored      int           `json:"ignored"`
	Baselined    int           `json:"baselined"`
	Sampled      []jsonSkipped `json:"sampled,omitempty"`
	Columns      []jsonColumn  `json:"columns,omitempty"`
}

// jsonColumn counts the findings in one column of one data file or SQL table.
type jsonColumn struct {
	Path       string         `json:"path"`
	ColumnName string         `json:"column_name"`
	Findings   int            `json:"findings"`
	Detectors  []jsonDetector `json:"detectors"`
}

type jsonDetector struct {
	Detector string `json:"detector"`
	Findings int    `json:"findings"`
}

type jsonSkipped struct {
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	NotRead bool   `json:"not_read,omitempty"`
}

type jsonFinding struct {
	Kind            string     `json:"kind"`
	SpecialCategory string     `json:"special_category,omitempty"`
	Sink            string     `json:"sink,omitempty"`
	DataCategory    string     `json:"data_category,omitempty"`
	Detector        string     `json:"detector"`
	Label           string     `json:"label"`
	Country         string     `json:"country"`
	Path            string     `json:"path"`
	Line            int        `json:"line"`
	Column          int        `json:"column"`
	ColumnName      string     `json:"column_name,omitempty"`
	Confidence      string     `json:"confidence"`
	Masked          string     `json:"masked"`
	Reasons         []string   `json:"reasons"`
	Sample          bool       `json:"sample"`
	TestPath        bool       `json:"test_path"`
	Fingerprint     string     `json:"fingerprint"`
	Store           *jsonStore `json:"store,omitempty"`
}

// jsonStore names the data store an infra finding is about.
type jsonStore struct {
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Service      string   `json:"service"`
	Regions      []string `json:"regions"`
	PersonalData bool     `json:"personal_data"`
}

func storeRef(s *domain.Store) *jsonStore {
	if s == nil {
		return nil
	}
	return &jsonStore{Type: s.Type, Name: s.Name, Provider: s.Provider, Service: s.Service, Regions: nonNilStrings(s.Regions), PersonalData: s.PersonalData}
}

func writeJSON(w io.Writer, r Report, _ Options) error {
	out := jsonReport{
		SchemaVersion: SchemaVersion,
		Tool:          jsonTool{Name: r.Tool, Version: r.Version},
		Summary: jsonSummary{
			Findings:     len(r.Findings),
			FilesScanned: r.Summary.FilesScanned,
			FilesSkipped: nonNil(r.Summary.FilesSkipped),
			Unreadable:   nonNil(r.Summary.Unreadable),
			Ignored:      r.Summary.Ignored,
			Baselined:    r.Summary.Baselined,
			Sampled:      optional(r.Summary.Sampled),
			Columns:      columns(r.Findings),
		},
		Findings: make([]jsonFinding, 0, len(r.Findings)),
	}
	for _, f := range r.Findings {
		kind := f.Kind
		if kind == "" {
			kind = domain.KindValue
		}
		out.Findings = append(out.Findings, jsonFinding{
			Kind: string(kind), SpecialCategory: string(f.Special), Sink: string(f.Sink), DataCategory: string(f.Category),
			Detector: f.Detector, Label: f.Label, Country: f.Country,
			Path: f.Location.Path, Line: f.Location.Line, Column: f.Location.Column, ColumnName: f.ColumnName,
			Confidence: f.Confidence.String(), Masked: f.Masked, Reasons: nonNilStrings(f.Reasons),
			Sample: f.Sample, TestPath: f.TestPath, Fingerprint: f.Fingerprint, Store: storeRef(f.Store),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

// columns groups the findings that have a column by file and column, in order of appearance.
func columns(fs []domain.Finding) []jsonColumn {
	var out []jsonColumn
	index := map[[2]string]int{}
	for _, f := range fs {
		if f.ColumnName == "" {
			continue
		}
		key := [2]string{f.Location.Path, f.ColumnName}
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, jsonColumn{Path: f.Location.Path, ColumnName: f.ColumnName})
		}
		c := &out[i]
		c.Findings++
		found := false
		for k := range c.Detectors {
			if c.Detectors[k].Detector == f.Detector {
				c.Detectors[k].Findings++
				found = true
			}
		}
		if !found {
			c.Detectors = append(c.Detectors, jsonDetector{Detector: f.Detector, Findings: 1})
		}
	}
	return out
}

func optional(s []domain.Skipped) []jsonSkipped {
	if len(s) == 0 {
		return nil
	}
	return nonNil(s)
}

func nonNil(s []domain.Skipped) []jsonSkipped {
	out := make([]jsonSkipped, 0, len(s))
	for _, k := range s {
		out = append(out, jsonSkipped{Path: k.Path, Reason: k.Reason, NotRead: k.NotRead})
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
