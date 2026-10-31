package sinks

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Session replay tools (Hotjar, FullStory, LogRocket, Microsoft Clarity, Smartlook, PostHog, Sentry Replay,
// Datadog RUM) record what people type into forms unless inputs are masked. Most mask inputs by default, so a
// finding is the code that switches masking off: a setting for every input, or an attribute that lets one
// form field be recorded.

// replayExts are the files where replay settings and form markup are written.
var replayExts = map[string]bool{
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".mts": true, ".cts": true,
	".html": true, ".htm": true, ".vue": true, ".svelte": true, ".astro": true, ".ejs": true, ".hbs": true,
	".erb": true, ".twig": true, ".liquid": true, ".njk": true, ".cshtml": true, ".php": true,
}

// replaySetting is a setting that makes a replay tool record every form input or all page text.
type replaySetting struct {
	re   *regexp.Regexp
	what string
}

var replaySettings = []replaySetting{
	{regexp.MustCompile(`\bmaskAllInputs["']?\s*:\s*false\b`), "masking of form inputs is switched off"},
	{regexp.MustCompile(`\bmaskAllText["']?\s*:\s*false\b`), "masking of page text is switched off"},
	{regexp.MustCompile(`\bmask_all_inputs["']?\s*:\s*false\b`), "masking of form inputs is switched off"},
	{regexp.MustCompile(`\bdefaultPrivacyLevel["']?\s*:\s*["']allow["']`), "the privacy level records every input"},
	{regexp.MustCompile(`(?i:smartlook)\W[^\n]*?\brecord\b[^\n]*?\b(?:forms|emails|numbers)["']?\s*:\s*true\b`), "Smartlook is told to record form fields"},
}

// replayWords must appear on a line before the settings are matched.
var replayWords = []string{"maskall", "mask_all", "defaultprivacylevel", "smartlook"}

// unmask is an attribute that lets a replay tool record one element; it counts on a form field only.
var unmask = regexp.MustCompile(`\bdata-hj-allow\b|\bdata-clarity-unmask\s*=\s*["']?true|\bdata-dd-privacy\s*=\s*["']allow|\bclass(?:Name)?\s*=\s*["'][^"']*\bfs-unmask\b`)

var (
	formField = regexp.MustCompile(`<(?i:input|textarea|select)\b`)
	fieldName = regexp.MustCompile(`\b(?:name|id|autocomplete)\s*=\s*["']([\w.\-\[\]]+)["']`)
	fieldType = regexp.MustCompile(`\btype\s*=\s*["'](email|tel|password|date)["']`)
)

// replayLine reports the replay settings and unmasked form fields on one line.
func (s *Scanner) replayLine(number int, line string) []domain.Finding {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "<!--") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "#") {
		return nil
	}
	lower := strings.ToLower(line)
	var out []domain.Finding
	for _, w := range replayWords {
		if !strings.Contains(lower, w) {
			continue
		}
		for _, rs := range replaySettings {
			if m := rs.re.FindStringIndex(line); m != nil {
				out = append(out, s.replayFinding(line, number, m[0], m[1], verdict{confidence: domain.Medium, reason: rs.what}, "the replay records whatever people type"))
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Location.Column < out[j].Location.Column })
		break
	}
	m := unmask.FindStringIndex(line)
	if m == nil || !formField.MatchString(line) {
		return out
	}
	v := verdict{confidence: domain.Low, reason: "a form field is let out of input masking"}
	if f := fieldName.FindStringSubmatch(line); f != nil {
		fv, ok := field(nil, f[1])
		if !ok {
			return out
		}
		fv.reason = fmt.Sprintf("a form field named %q is let out of input masking", f[1])
		v = fv
	} else if f := fieldType.FindStringSubmatch(line); f != nil {
		v = verdict{category: domain.CategoryContact, confidence: domain.Medium, reason: fmt.Sprintf("a form field of type %s is let out of input masking", f[1])}
		if f[1] == "date" {
			v.category = domain.CategoryBirth
			v.confidence = domain.Low
		}
	}
	return append(out, s.replayFinding(line, number, m[0], m[1], v, "the replay records what is typed into it"))
}

func (s *Scanner) replayFinding(line string, number, start, end int, v verdict, effect string) domain.Finding {
	return domain.Finding{
		Kind:       domain.KindSink,
		Category:   v.category,
		Special:    v.special,
		Sink:       domain.SinkSessionReplay,
		Detector:   Detector,
		Label:      Labels[domain.SinkSessionReplay],
		Country:    domain.CrossBorder,
		Location:   domain.Location{Path: s.path, Line: number, Column: utf8.RuneCountInString(line[:start]) + 1},
		Masked:     strings.Join(strings.Fields(line[start:end]), " "),
		Confidence: v.confidence,
		Reasons:    []string{v.reason, effect},
	}
}

func replayFile(p string) bool { return replayExts[strings.ToLower(path.Ext(p))] }
