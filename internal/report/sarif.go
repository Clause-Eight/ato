package report

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

func init() { register("sarif", writeSARIF) }

const sarifSchema = "https://json.schemastore.org/sarif-2.1.0.json"

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifRule struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ShortDescription sarifText      `json:"shortDescription"`
	FullDescription  sarifText      `json:"fullDescription"`
	Help             sarifText      `json:"help"`
	Properties       map[string]any `json:"properties"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
	Properties          map[string]any    `json:"properties"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
}

func level(c domain.Confidence) string {
	switch c {
	case domain.High:
		return "error"
	case domain.Medium:
		return "warning"
	}
	return "note"
}

func writeSARIF(w io.Writer, r Report, _ Options) error {
	rules := map[string]sarifRule{}
	results := make([]sarifResult, 0, len(r.Findings))
	for _, f := range r.Findings {
		if _, ok := rules[ruleID(f)]; !ok {
			rules[ruleID(f)] = ruleFor(f)
		}
		results = append(results, sarifResult{
			RuleID:  ruleID(f),
			Level:   level(f.Confidence),
			Message: sarifText{message(f)},
			Locations: []sarifLocation{{PhysicalLocation: sarifPhysical{
				ArtifactLocation: sarifArtifact{URI: f.Location.Path, URIBaseID: "%SRCROOT%"},
				Region:           sarifRegion{StartLine: f.Location.Line, StartColumn: f.Location.Column},
			}}},
			PartialFingerprints: map[string]string{"piiEu/v1": f.Fingerprint},
			Properties:          map[string]any{"confidence": f.Confidence.String(), "sample": f.Sample, "testPath": f.TestPath},
		})
	}
	return writeLog(w, r, rules, results)
}

// ruleID is the detector name, with the Art. 9 category for special-category fields.
func ruleID(f domain.Finding) string {
	if f.Kind == domain.KindField {
		return f.Detector + "/" + string(f.Special)
	}
	return f.Detector
}

func ruleFor(f domain.Finding) sarifRule {
	if f.Kind == domain.KindField {
		return sarifRule{
			ID:               ruleID(f),
			Name:             ruleName(f.Label),
			ShortDescription: sarifText{f.Label},
			FullDescription:  sarifText{f.Label + ". Processing special categories of personal data is prohibited unless an Art. 9(2) condition applies."},
			Help:             sarifText{"Record the Art. 9(2) condition and the safeguards for this field, or remove it if the data is not needed."},
			Properties:       map[string]any{"tags": []string{"privacy", "gdpr", "gdpr-art-9", string(f.Special)}},
		}
	}
	if f.Kind == domain.KindStore {
		return sarifRule{
			ID:               ruleID(f),
			Name:             ruleName(f.Label),
			ShortDescription: sarifText{f.Label},
			FullDescription:  sarifText{f.Label + ". The code has no path that removes this data, so it may be kept for ever or be impossible to erase on request."},
			Help:             sarifText{"Add a delete or anonymising update, a retention job or a TTL for this store, or record where it happens outside the code."},
			Properties:       map[string]any{"tags": []string{"privacy", "gdpr", "data-lifecycle"}},
		}
	}
	return sarifRule{
		ID:               f.Detector,
		Name:             ruleName(f.Label),
		ShortDescription: sarifText{f.Label},
		FullDescription:  sarifText{f.Label + " found in the repository. Personal data in code, fixtures and logs is easy to copy and hard to delete."},
		Help:             sarifText{"Replace the value with synthetic test data or a reference, and remove it from history if it is real."},
		Properties:       map[string]any{"tags": []string{"privacy", "gdpr", strings.ToLower(f.Country)}},
	}
}

func writeLog(w io.Writer, r Report, rules map[string]sarifRule, results []sarifResult) error {
	ids := make([]string, 0, len(rules))
	for id := range rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	driver := sarifDriver{Name: r.Tool, Version: r.Version, InformationURI: "https://github.com/Clause-Eight/pii-eu", Rules: make([]sarifRule, 0, len(ids))}
	for _, id := range ids {
		driver.Rules = append(driver.Rules, rules[id])
	}
	log := sarifLog{Schema: sarifSchema, Version: "2.1.0", Runs: []sarifRun{{Tool: sarifTool{Driver: driver}, Results: results}}}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(log)
}

func message(f domain.Finding) string {
	msg := f.Label + " " + f.Masked + " (" + f.Confidence.String() + " confidence)"
	if len(f.Reasons) > 0 {
		msg += ": " + strings.Join(f.Reasons, "; ")
	}
	return msg
}

// ruleName turns a label into the PascalCase name SARIF viewers expect: "Payment card number" -> "PaymentCardNumber".
func ruleName(label string) string {
	var b strings.Builder
	for _, w := range strings.FieldsFunc(label, func(r rune) bool { return r == ' ' || r == '(' || r == ')' || r == '-' }) {
		r := []rune(w)
		b.WriteString(strings.ToUpper(string(r[0])) + string(r[1:]))
	}
	return b.String()
}
