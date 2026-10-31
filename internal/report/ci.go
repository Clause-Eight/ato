package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

func init() {
	register("github", writeGitHub)
	register("codequality", writeCodeQuality)
}

// writeGitHub prints GitHub Actions workflow commands, which show as annotations on the pull request.
func writeGitHub(w io.Writer, r Report, _ Options) error {
	var b strings.Builder
	for _, f := range r.Findings {
		cmd := map[domain.Confidence]string{domain.High: "error", domain.Medium: "warning"}[f.Confidence]
		if cmd == "" {
			cmd = "notice"
		}
		fmt.Fprintf(&b, "::%s file=%s,line=%d,col=%d,title=%s::%s\n",
			cmd, escapeProperty(f.Location.Path), f.Location.Line, f.Location.Column, escapeProperty("ato: "+f.Label), escapeData(message(f)))
	}
	fmt.Fprintf(&b, "%s\n", summaryLine(r, countFiles(r.Findings)))
	_, err := io.WriteString(w, b.String())
	return err
}

func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func escapeProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

func countFiles(fs []domain.Finding) int {
	seen := map[string]bool{}
	for _, f := range fs {
		seen[f.Location.Path] = true
	}
	return len(seen)
}

type codeQualityIssue struct {
	Description string              `json:"description"`
	CheckName   string              `json:"check_name"`
	Fingerprint string              `json:"fingerprint"`
	Severity    string              `json:"severity"`
	Location    codeQualityLocation `json:"location"`
}

type codeQualityLocation struct {
	Path  string           `json:"path"`
	Lines codeQualityLines `json:"lines"`
}

type codeQualityLines struct {
	Begin int `json:"begin"`
}

// writeCodeQuality writes a GitLab Code Quality report, shown in the merge request widget.
func writeCodeQuality(w io.Writer, r Report, _ Options) error {
	out := make([]codeQualityIssue, 0, len(r.Findings))
	for _, f := range r.Findings {
		sev := map[domain.Confidence]string{domain.High: "critical", domain.Medium: "major", domain.Low: "minor"}[f.Confidence]
		out = append(out, codeQualityIssue{
			Description: message(f),
			CheckName:   "ato/" + f.Detector,
			Fingerprint: f.Fingerprint,
			Severity:    sev,
			Location:    codeQualityLocation{Path: f.Location.Path, Lines: codeQualityLines{Begin: f.Location.Line}},
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}
