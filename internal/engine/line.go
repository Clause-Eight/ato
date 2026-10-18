package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
)

type lineContext struct {
	path     string
	testPath bool
	number   int
	line     string
	previous string
}

type match struct {
	det        detectors.Detector
	start, end int
	value      string
	confidence domain.Confidence
	reasons    []string
	sample     bool
}

var longNumber = regexp.MustCompile(`\d{10,}`)

// numberTable reports whether a line holds three or more long numbers, like a table of constants.
func numberTable(line string) bool {
	return len(longNumber.FindAllStringIndex(line, 3)) >= 3
}

var testPathRe = regexp.MustCompile(`(?i)(^|/)(tests?|__tests__|spec|specs|fixtures?|testdata|test_data|examples?|samples?|mocks?|__mocks__|stubs?)(/|$)|(^|/)[^/]*([._-](test|spec)s?\.[a-z0-9]+|test_[^/]*)$`)

// IsTestPath reports whether a path looks like tests, fixtures or examples.
func IsTestPath(path string) bool {
	return testPathRe.MatchString(path)
}

func scanLine(ctx lineContext, dets []detectors.Detector) []domain.Finding {
	if ctx.line == "" {
		return nil
	}
	lower := strings.ToLower(ctx.previous + "\n" + ctx.line)
	var matches []match
	for _, d := range dets {
		for _, loc := range d.Pattern.FindAllStringIndex(ctx.line, -1) {
			if m, ok := evaluate(ctx, d, loc[0], loc[1], lower); ok {
				matches = append(matches, m)
			}
		}
	}
	matches = dedupe(matches)
	findings := toFindings(ctx, matches)
	applyIgnore(findings, ctx)
	return findings
}

const ignoreMarker = "pii-eu:ignore"

// applyIgnore honours `pii-eu:ignore <reason>` on the same line or the line before. Without a reason the comment does not apply.
func applyIgnore(fs []domain.Finding, ctx lineContext) {
	if len(fs) == 0 {
		return
	}
	found, reason := ignoreReason(ctx.line)
	if !found {
		found, reason = ignoreReason(ctx.previous)
	}
	if !found {
		return
	}
	for i := range fs {
		if reason == "" {
			fs[i].Reasons = append(fs[i].Reasons, "the ignore comment has no reason, so it does not apply")
			continue
		}
		fs[i].Ignored = true
	}
}

func ignoreReason(line string) (bool, string) {
	i := strings.Index(line, ignoreMarker)
	if i < 0 {
		return false, ""
	}
	rest := line[i+len(ignoreMarker):]
	for _, end := range []string{"*/", "-->", "#}", "%}"} {
		if j := strings.Index(rest, end); j >= 0 {
			rest = rest[:j]
		}
	}
	return true, strings.TrimSpace(strings.TrimLeft(rest, " :\t-"))
}

func evaluate(ctx lineContext, d detectors.Detector, start, end int, lower string) (match, bool) {
	if !bounded(ctx.line, start, end) {
		return match{}, false
	}
	end, ok := validEnd(ctx.line, d, start, end)
	if !ok {
		return match{}, false
	}
	value := d.Normalize(ctx.line[start:end])
	m := match{det: d, start: start, end: end, value: value, confidence: d.Base}
	if d.Validate != nil {
		m.reasons = append(m.reasons, d.Validation)
	}
	if kw := keyword(lower, d.Keywords); kw != "" {
		m.confidence = raise(m.confidence)
		m.reasons = append(m.reasons, fmt.Sprintf("the word %q is nearby", kw))
	}
	if d.Samples[value] {
		m.sample = true
		m.confidence = domain.Low
		m.reasons = append(m.reasons, "a published sample value")
	}
	if !m.sample && numberTable(ctx.line) {
		m.confidence = lower1(m.confidence)
		m.reasons = append(m.reasons, "in a line of long numbers, which is usually data such as seeds or constants")
	}
	if ctx.testPath && !m.sample {
		m.confidence = lower1(m.confidence)
		m.reasons = append(m.reasons, "in a test, fixture or example path")
	}
	return m, true
}

// validEnd returns where a valid value ends, trimming trailing groups when the detector allows it.
func validEnd(line string, d detectors.Detector, start, end int) (int, bool) {
	for {
		if d.Validate == nil || d.Validate(d.Normalize(line[start:end])) {
			return end, true
		}
		if !d.TrimGroups {
			return 0, false
		}
		cut := strings.LastIndexAny(line[start:end], " -")
		if cut <= 0 {
			return 0, false
		}
		end = start + cut
	}
}

// bounded rejects a candidate that is part of a longer token, including the digits of a decimal number
// such as 7.6971174701310497 or 1,234567890123.
func bounded(line string, start, end int) bool {
	if start > 0 {
		r, n := utf8.DecodeLastRuneInString(line[:start])
		if isAlnum(r) || isDecimalMark(r) && start-n > 0 && isDigit(line[start-n-1]) || r == '-' && isSign(line, start-n) {
			return false
		}
	}
	if end < len(line) {
		r, n := utf8.DecodeRuneInString(line[end:])
		if isAlnum(r) || isDecimalMark(r) && end+n < len(line) && isDigit(line[end+n]) {
			return false
		}
	}
	return true
}

func isDecimalMark(r rune) bool { return r == '.' || r == ',' }

// isSign reports whether the minus at i is a number's sign: at the start, or after a space, bracket, comma or operator.
func isSign(line string, i int) bool {
	if i == 0 {
		return true
	}
	return strings.ContainsRune(" \t([{,=:", rune(line[i-1]))
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// keyword returns the first word found as a whole word; underscores and punctuation count as word breaks.
func keyword(lower string, words []string) string {
	for _, w := range words {
		for from := 0; ; {
			i := strings.Index(lower[from:], w)
			if i < 0 {
				break
			}
			i += from
			if wordBreak(lower, i, i+len(w)) {
				return w
			}
			from = i + 1
		}
	}
	return ""
}

func wordBreak(s string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if unicode.IsLetter(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func raise(c domain.Confidence) domain.Confidence {
	if c < domain.High {
		return c + 1
	}
	return c
}

func lower1(c domain.Confidence) domain.Confidence {
	if c > domain.Low {
		return c - 1
	}
	return c
}

// dedupe keeps one match per overlapping span: the most confident, then the most specific base.
func dedupe(ms []match) []match {
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].confidence != ms[j].confidence {
			return ms[i].confidence > ms[j].confidence
		}
		if ms[i].det.Base != ms[j].det.Base {
			return ms[i].det.Base > ms[j].det.Base
		}
		return ms[i].end-ms[i].start > ms[j].end-ms[j].start
	})
	var kept []match
	for _, m := range ms {
		overlaps := false
		for _, k := range kept {
			if m.start < k.end && k.start < m.end {
				overlaps = true
				break
			}
		}
		if !overlaps {
			kept = append(kept, m)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].start < kept[j].start })
	return kept
}

func toFindings(ctx lineContext, ms []match) []domain.Finding {
	if len(ms) == 0 {
		return nil
	}
	shape := lineShape(ctx.line, ms)
	out := make([]domain.Finding, 0, len(ms))
	for i, m := range ms {
		out = append(out, domain.Finding{
			Detector:    m.det.Name,
			Label:       m.det.Label,
			Country:     m.det.Country,
			Location:    domain.Location{Path: ctx.path, Line: ctx.number, Column: utf8.RuneCountInString(ctx.line[:m.start]) + 1},
			Value:       m.value,
			Masked:      m.det.Mask(m.value),
			Confidence:  m.confidence,
			Reasons:     m.reasons,
			Sample:      m.sample,
			TestPath:    ctx.testPath,
			Fingerprint: fingerprint(m.det.Name, ctx.path, shape, i),
		})
	}
	return out
}

// lineShape is the line with every found value replaced, so fingerprints never depend on the value itself.
func lineShape(line string, ms []match) string {
	var b strings.Builder
	last := 0
	for _, m := range ms {
		b.WriteString(line[last:m.start])
		b.WriteString("<" + m.det.Name + ">")
		last = m.end
	}
	b.WriteString(line[last:])
	return strings.TrimSpace(b.String())
}

func fingerprint(detector, path, shape string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d", detector, path, shape, index)))
	return hex.EncodeToString(sum[:16])
}
