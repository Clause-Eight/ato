package report

import (
	"encoding/json"
	"io"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// SchemaVersion is the version of the JSON report. A new optional field is a minor bump; a removed or changed field is a major bump.
const SchemaVersion = "1.0"

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
}

type jsonSkipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type jsonFinding struct {
	Detector    string   `json:"detector"`
	Label       string   `json:"label"`
	Country     string   `json:"country"`
	Path        string   `json:"path"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	Confidence  string   `json:"confidence"`
	Masked      string   `json:"masked"`
	Reasons     []string `json:"reasons"`
	Sample      bool     `json:"sample"`
	TestPath    bool     `json:"test_path"`
	Fingerprint string   `json:"fingerprint"`
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
		},
		Findings: make([]jsonFinding, 0, len(r.Findings)),
	}
	for _, f := range r.Findings {
		out.Findings = append(out.Findings, jsonFinding{
			Detector: f.Detector, Label: f.Label, Country: f.Country,
			Path: f.Location.Path, Line: f.Location.Line, Column: f.Location.Column,
			Confidence: f.Confidence.String(), Masked: f.Masked, Reasons: nonNilStrings(f.Reasons),
			Sample: f.Sample, TestPath: f.TestPath, Fingerprint: f.Fingerprint,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

func nonNil(s []domain.Skipped) []jsonSkipped {
	out := make([]jsonSkipped, 0, len(s))
	for _, k := range s {
		out = append(out, jsonSkipped{Path: k.Path, Reason: k.Reason})
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
