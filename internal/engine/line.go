package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

type lineContext struct {
	path     string
	testPath bool
	number   int
	line     string
	previous string
	segments []segment
	folded   *string
	uuids    *[][]int
	record   *string
	own      *string
}

// segment is one value of a line in a file with columns: a CSV cell, a JSON Lines value or a SQL value. Its
// column name, not the rest of the line, is the keyword context. A line without segments is scanned whole.
type segment struct {
	start, end int
	column     string
	keywords   string
}

// keywordText is the previous and current line folded for keyword matching, computed once per line on first use.
func (c *lineContext) keywordText() string {
	if c.folded == nil {
		f := detectors.Fold(c.previous + "\n" + c.line)
		c.folded = &f
	}
	return *c.folded
}

// lineText is the current line alone folded for keyword matching, computed once per line on first use.
func (c *lineContext) lineText() string {
	if c.own == nil {
		f := detectors.Fold(c.line)
		c.own = &f
	}
	return *c.own
}

// keywordReach is how far, in bytes, a keyword may sit before or after a value that needs its keyword close by.
// It spans a key and its separator, as in "passport_number": , and keeps a word elsewhere on a long minified
// line from counting.
const keywordReach = 48

// near is the text around line[start:end] within keywordReach, cut at character boundaries.
func near(line string, start, end int) string {
	from, to := max(0, start-keywordReach), min(len(line), end+keywordReach)
	for from > 0 && !utf8.RuneStart(line[from]) {
		from--
	}
	for to < len(line) && !utf8.RuneStart(line[to]) {
		to++
	}
	return line[from:to]
}

type match struct {
	det        detectors.Detector
	start, end int
	value      string
	confidence domain.Confidence
	reasons    []string
	sample     bool
	keyword    bool
	ownWord    bool
	column     string
	possible   []detectors.Detector
}

// personalField finds a field name that holds personal data in the same record: a name, an address, a birth
// date or a contact detail, as a key in JSON, YAML, a dict, a hash or a column (#269).
var personalField = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])(first_?name|last_?name|surname|given_?name|family_?name|full_?name|date_?of_?birth|birth_?date|dob|street|address_?line\d?|postal_?code|post_?code|zip_?code|nationality|gender)["']?\s*(?::|=>|=)`)

var uuidShape = regexp.MustCompile(`[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}`)

// inUUID reports whether a candidate lies inside a UUID, whose hex groups can pass an identifier's check. The
// UUIDs of a line are found once, on first use.
func (c *lineContext) inUUID(start, end int) bool {
	if c.uuids == nil {
		var locs [][]int
		if strings.Count(c.line, "-") >= 4 {
			locs = uuidShape.FindAllStringIndex(c.line, -1)
		}
		c.uuids = &locs
	}
	for _, loc := range *c.uuids {
		if start <= loc[0] && end >= loc[1] {
			continue // the candidate is the whole UUID, as an advertising ID is
		}
		if start < loc[1] && loc[0] < end {
			return true
		}
	}
	return false
}

// ownKey is the key written just before a value: `phone: "…"`, `'postal_code' => '…'`, `"height": …`, `zip=…`.
var ownKey = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_.-]*)\\?["']?\s*(?::|=>|=)\s*\\?["']?$`)

// otherKinds are key names that say a number is something other than an identifier, with the detectors that
// may still match under them. A national number under `phone:` is a phone number that happens to pass a check.
var otherKinds = []struct {
	words  []string
	allows string
}{
	{[]string{"phone", "telephone", "tel", "mobile", "cell", "fax", "msisdn", "phone_number", "phonenumber"}, "eu-phone"},
	{[]string{"zip", "zipcode", "zip_code", "postal_code", "postalcode", "postcode", "post_code"}, "postcode"},
	{[]string{"height", "width", "weight", "length", "depth", "size", "quantity", "qty", "price", "amount", "total", "version", "port", "timestamp", "duration"}, ""},
}

// otherKind returns the key before a value when it names another kind of value than detector name.
func otherKind(before, name string) string {
	if len(before) > 80 {
		before = before[len(before)-80:]
	}
	m := ownKey.FindStringSubmatch(before)
	if m == nil {
		return ""
	}
	key := strings.ToLower(m[1])
	if i := strings.LastIndexAny(key, "."); i >= 0 {
		key = key[i+1:]
	}
	for _, k := range otherKinds {
		if k.allows == name {
			continue
		}
		for _, w := range k.words {
			if key == w || strings.HasSuffix(key, "_"+w) {
				return key
			}
		}
	}
	return ""
}

// recordField returns a personal field named in a value's record: the line and the line before (found once per
// line), or a segment's column name in a file with columns.
func (c *lineContext) recordField(seg *segment) string {
	if seg != nil {
		if f := personalField.FindStringSubmatch(seg.column + ":"); f != nil {
			return f[1]
		}
		return ""
	}
	if c.record == nil {
		var r string
		if f := personalField.FindStringSubmatch(c.previous + "\n" + c.line); f != nil {
			r = f[1]
		}
		c.record = &r
	}
	return *c.record
}

// timeLabel finds names that usually hold a time: created_at, updatedAt, ts, timestamp, time, date.
var timeLabel = regexp.MustCompile(`(?i)(_at|At|_on|_ts)\b|\b(ts|time|timestamp|date|datetime|epoch|millis|ms)\b`)

// millisTimestamp reports whether a value is 13 digits that read as a Unix time in milliseconds between 2000 and 2100.
func millisTimestamp(v string) bool {
	if len(v) != 13 {
		return false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	return err == nil && n >= 946684800000 && n < 4102444800000
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
	var matches []match
	if ctx.segments == nil {
		matches = scanSegment(&ctx, nil, 0, len(ctx.line), dets, matches)
		matches = decodedMatches(&ctx, nil, 0, len(ctx.line), dets, matches)
	}
	for i := range ctx.segments {
		s := &ctx.segments[i]
		matches = scanSegment(&ctx, s, s.start, s.end, dets, matches)
		matches = decodedMatches(&ctx, s, s.start, s.end, dets, matches)
	}
	matches = dedupe(matches)
	findings := toFindings(ctx, matches)
	applyIgnore(findings, ctx)
	return findings
}

// scanSegment runs the detectors whose prefilter passes over line[start:end]; seg is nil for a whole line.
func scanSegment(ctx *lineContext, seg *segment, start, end int, dets []detectors.Detector, matches []match) []match {
	text := ctx.line[start:end]
	for _, d := range candidates(text, countDigits(text), dets) {
		for _, loc := range d.Pattern.FindAllStringIndex(text, -1) {
			if m, ok := evaluate(ctx, seg, d, start+loc[0], start+loc[1]); ok {
				matches = append(matches, m)
			}
		}
	}
	return matches
}

// candidates are the detectors whose prefilter the line passes; only their patterns run.
func candidates(line string, digits int, dets []detectors.Detector) []detectors.Detector {
	out := make([]detectors.Detector, 0, len(dets))
	for _, d := range dets {
		if d.Needs.Admits(line, digits) {
			out = append(out, d)
		}
	}
	return out
}

func countDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if isDigit(s[i]) {
			n++
		}
	}
	return n
}

const ignoreMarker = "ato:ignore"

// legacyIgnoreMarker is the form written before the rename to ATO. It is deprecated and still honoured in 2.x.
const legacyIgnoreMarker = "pii-eu:ignore"

// applyIgnore honours `ato:ignore <reason>`, or the deprecated `pii-eu:ignore <reason>`, on the same line or the
// line before. Without a reason the comment does not apply.
func applyIgnore(fs []domain.Finding, ctx lineContext) {
	if len(fs) == 0 {
		return
	}
	found, legacy, reason := ignoreReason(ctx.line)
	if !found {
		found, legacy, reason = ignoreReason(ctx.previous)
	}
	if !found {
		return
	}
	for i := range fs {
		fs[i].LegacyIgnore = legacy
		if reason == "" {
			fs[i].Reasons = append(fs[i].Reasons, "the ignore comment has no reason, so it does not apply")
			continue
		}
		fs[i].Ignored = true
	}
}

func ignoreReason(line string) (found, legacy bool, reason string) {
	i, n := markerIndex(line, ignoreMarker), len(ignoreMarker)
	if i < 0 {
		i, n, legacy = strings.Index(line, legacyIgnoreMarker), len(legacyIgnoreMarker), true
	}
	if i < 0 {
		return false, false, ""
	}
	rest := line[i+n:]
	for _, end := range []string{"*/", "-->", "#}", "%}"} {
		if j := strings.Index(rest, end); j >= 0 {
			rest = rest[:j]
		}
	}
	return true, legacy, strings.TrimSpace(strings.TrimLeft(rest, " :\t-"))
}

// markerIndex finds marker where it starts a word, so that a longer word ending in "ato" does not count.
func markerIndex(line, marker string) int {
	for from := 0; ; {
		i := strings.Index(line[from:], marker)
		if i < 0 {
			return -1
		}
		i += from
		if i == 0 || !isWordByte(line[i-1]) {
			return i
		}
		from = i + 1
	}
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// evaluate checks one candidate. In a segment the column name gives the keywords and the segment is the text
// judged as a table of numbers; otherwise the line and the line before do.
func evaluate(ctx *lineContext, seg *segment, d detectors.Detector, start, end int) (match, bool) {
	if ctx.inUUID(start, end) {
		return match{}, false
	}
	if seg != nil && !bounded(ctx.line[seg.start:seg.end], start-seg.start, end-seg.start) || seg == nil && !bounded(ctx.line, start, end) {
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
	if d.Formatted != nil && d.Formatted.MatchString(ctx.line[start:end]) {
		m.confidence = raise(m.confidence)
		m.reasons = append(m.reasons, "written in the identifier's usual format")
	}
	words := d.MatchWords
	if words == nil {
		words = d.Keywords
	}
	context, own, text := "", "", ctx.line
	if seg != nil {
		context, own, text, m.column = seg.keywords, seg.keywords, ctx.line[seg.start:seg.end], seg.column
	} else {
		context, own = ctx.keywordText(), ctx.lineText()
		if d.KeywordOnLine {
			context = detectors.Fold(near(ctx.line, start, end))
		}
	}
	if r := d.Rival; r != nil && r.Valid(value) && keyword(own, r.Words) != "" {
		return match{}, false
	}
	kw := keyword(context, words)
	if kw == "" && d.NeedsKeyword {
		return match{}, false
	}
	// A keyword counts on the line before too; a word that labels other numbers only on the value's own line.
	// Context can lower a valid value by one step at most, however many signals agree; every signal is still
	// listed as a reason.
	demoted := false
	demote := func(reason string) {
		if !demoted {
			m.confidence = lower1(m.confidence)
			demoted = true
		}
		m.reasons = append(m.reasons, reason)
	}
	if kw != "" {
		m.confidence = raise(m.confidence)
		m.reasons = append(m.reasons, fmt.Sprintf("the word %q is nearby", kw))
		m.keyword = true
		m.ownWord = contains(d.OwnWords, kw) || d.OwnWords == nil
	} else if w := keyword(own, d.Unlikely); w != "" {
		demote(fmt.Sprintf("the word %q is nearby, which usually labels other numbers", w))
	} else if k := otherKind(ctx.line[:start], d.Name); k != "" {
		demote(fmt.Sprintf("its own name %q says it is another kind of value", k))
	} else if distinct[d.Name] && wholeQuoted(ctx.line, start, end) && !demoted {
		m.confidence = raise(m.confidence)
		m.reasons = append(m.reasons, "the whole of a quoted value")
	} else if f := ctx.recordField(seg); f != "" {
		m.confidence = raise(m.confidence)
		m.reasons = append(m.reasons, fmt.Sprintf("next to another personal field (%s), as in a record about a person", f))
	}
	if d.Samples[value] {
		m.sample = true
		m.confidence = domain.Low
		m.reasons = append(m.reasons, "a published sample value")
	} else if detectors.Synthetic(ctx.line[start:end]) {
		m.sample = true
		m.confidence = domain.Low
		m.reasons = append(m.reasons, "carries the mark of a synthetic value written by ato fix")
	}
	if m.sample {
		return m, true
	}
	if kw == "" && millisTimestamp(value) && timeLabel.MatchString(text) {
		demote("a time in milliseconds under a time-like name, such as created_at")
	}
	if numberTable(text) {
		demote("in a line of long numbers, which is usually data such as seeds or constants")
	}
	if ctx.testPath {
		// A test path is where real data copied from production does harm, so it is noted but does not lower a
		// value that passed its own rules (#269). Data without a check digit (names, addresses, dates of birth,
		// postcodes: the detectors that are off by default or need a keyword) is still lowered, since test data is
		// full of made-up ones.
		if d.NeedsKeyword || !d.OnByDefault {
			demote("in a test, fixture or example path")
		} else {
			m.reasons = append(m.reasons, "in a test, fixture or example path")
		}
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

// dedupe keeps one match per overlapping span: the most confident, then the most specific base. A match on the
// same span that is just as confident, with the same keyword evidence, is listed as possible: nothing near the
// value says which identifier it is.
func dedupe(ms []match) []match {
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].confidence != ms[j].confidence {
			return ms[i].confidence > ms[j].confidence
		}
		if ms[i].ownWord != ms[j].ownWord {
			return ms[i].ownWord
		}
		if ms[i].det.Base != ms[j].det.Base {
			return ms[i].det.Base > ms[j].det.Base
		}
		return ms[i].end-ms[i].start > ms[j].end-ms[j].start
	})
	var kept []match
	for _, m := range ms {
		overlaps := false
		for i, k := range kept {
			if m.start < k.end && k.start < m.end {
				overlaps = true
				switch {
				case m.start != k.start || m.end != k.end || m.det.Name == k.det.Name:
				case m.confidence == k.confidence && m.ownWord == k.ownWord && !m.sample && !k.sample:
					kept[i].possible = append(kept[i].possible, m.det)
					kept[i].reasons = append(kept[i].reasons, "just as valid as "+m.det.Label+"; nothing nearby says which")
				default:
					kept[i].reasons = append(kept[i].reasons, "also valid as "+m.det.Label+"; reported once, as the more likely")
				}
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
			Kind:        domain.KindValue,
			Detector:    m.det.Name,
			Label:       m.det.Label,
			Country:     m.det.Country,
			Location:    domain.Location{Path: ctx.path, Line: ctx.number, Column: utf8.RuneCountInString(ctx.line[:m.start]) + 1},
			ColumnName:  m.column,
			Span:        domain.Span{Start: m.start, End: m.end},
			Value:       m.value,
			Masked:      m.det.Mask(m.value),
			Confidence:  m.confidence,
			Reasons:     m.reasons,
			Sample:      m.sample,
			TestPath:    ctx.testPath,
			Keyword:     m.keyword,
			Fingerprint: fingerprint(m.det.Name, ctx.path, shape, i),
			Possible:    possible(m.possible),
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

func possible(ds []detectors.Detector) []domain.Possible {
	var out []domain.Possible
	for _, d := range ds {
		out = append(out, domain.Possible{Detector: d.Name, Label: d.Label, Country: d.Country})
	}
	return out
}

func fingerprint(detector, path, shape string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d", detector, path, shape, index)))
	return hex.EncodeToString(sum[:16])
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// distinct are identifiers that a random string of their shape rarely passes: a date of birth and a check digit,
// or a check letter. Only these are raised for being the whole of a quoted value; numbers with a plain checksum
// are not, since quoted codes in real data pass them often (#274).
var distinct = map[string]bool{"ro-cnp": true, "bg-egn": true, "pl-pesel": true, "es-dni": true, "es-nie": true,
	"it-fiscal-code": true, "fr-nir": true, "de-tax-id": true, "be-national-register": true, "ee-isikukood": true, "lt-asmens-kodas": true,
	"si-emso": true, "fi-hetu": true}

// wholeQuoted reports whether a value is the whole of a quoted string, as values in data and fixtures are.
func wholeQuoted(line string, start, end int) bool {
	if start == 0 || end >= len(line) {
		return false
	}
	q := line[start-1]
	return (q == '"' || q == '\'') && line[end] == q
}
