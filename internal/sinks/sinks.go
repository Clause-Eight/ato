// Package sinks finds calls that write personal data to logs, error trackers and analytics: a log, print,
// error-tracker or analytics call in Go, Python, JavaScript or TypeScript, Java, Kotlin, Ruby or PHP that is
// passed a field whose name says it holds personal data, a whole request body or a user-like object. It works
// line by line with patterns, not a parser, follows a value through one local variable in the same function,
// and leans towards missing a call rather than reporting one that is harmless.
package sinks

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Detector names sink findings. It can be switched off in settings like a detector.
const Detector = "log-sink"

// Labels are the finding labels per sink.
var Labels = map[domain.Sink]string{
	domain.SinkLog:          "Personal data written to a log",
	domain.SinkErrorTracker: "Personal data sent to an error tracker",
	domain.SinkAnalytics:    "Personal data sent to analytics",
}

var sinkWords = map[domain.Sink]string{
	domain.SinkLog:          "which writes to a log",
	domain.SinkErrorTracker: "which sends to an error tracker",
	domain.SinkAnalytics:    "which sends to analytics",
}

// maxLine skips longer lines, which are usually minified or generated code.
const maxLine = 4000

// maxContinuation is how many lines after its start a call's arguments are followed.
const maxContinuation = 10

// Scanner reads one file line by line, following a call's arguments onto the next lines.
type Scanner struct {
	path string
	lang language
	ok   bool
	open *call
	vars map[string]binding
}

// call is a sink call whose arguments are being read.
type call struct {
	sink   domain.Sink
	head   string
	depth  int
	bare   bool
	lines  int
	first  bool
	seen   map[string]bool
	closed bool
}

// NewScanner returns a scanner for the file at path. Files that are not source code in a known language
// produce no findings.
func NewScanner(path string) *Scanner {
	l, ok := languageOf(path)
	return &Scanner{path: path, lang: l, ok: ok}
}

// Line returns the sink findings on one line. Findings have no fingerprint, and their confidence does not
// yet account for test paths: the engine adds both.
func (s *Scanner) Line(number int, line string) []domain.Finding {
	if !s.ok {
		return nil
	}
	if len(line) > maxLine {
		s.open = nil
		return nil
	}
	var out []domain.Finding
	from := 0
	if s.open != nil {
		c := s.open
		s.open = nil
		c.lines++
		end := s.args(c, line, 0, number, &out)
		if !c.closed && !c.bare && c.lines < maxContinuation {
			s.open = c
			return out
		}
		from = end
	}
	if commentLine(line, s.lang) {
		return out
	}
	s.assignments(line, from, number)
	for _, h := range s.heads(line, from) {
		if s.open != nil {
			break
		}
		c := &call{sink: h.sink, head: h.name, depth: 1, bare: h.bare, first: true, seen: map[string]bool{}}
		if h.bare {
			c.depth = 0
		}
		s.args(c, line, h.end, number, &out)
		if !c.closed && !c.bare {
			s.open = c
		}
	}
	return out
}

type head struct {
	sink       domain.Sink
	name       string
	start, end int
	bare       bool
}

// heads finds the sink calls that start on a line, in order, one per call.
func (s *Scanner) heads(line string, from int) []head {
	if !strings.ContainsAny(line[from:], "({ ") {
		return nil
	}
	var hs []head
	lower := strings.ToLower(line[from:])
	for _, r := range s.lang.rules {
		if !r.admits(lower) {
			continue
		}
		for _, loc := range r.re.FindAllStringIndex(line[from:], -1) {
			start, end := loc[0]+from, loc[1]+from
			if c := line[start]; !isIdentStart(c) && c != '\\' && c != ' ' && c != '\t' {
				start++
			}
			for start < end && (line[start] == ' ' || line[start] == '\t') {
				start++
			}
			if definition.MatchString(line[:start]) || !inCode(line, start, s.lang) {
				continue
			}
			name := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[start:end]), "({"))
			bare := !strings.HasSuffix(strings.TrimSpace(line[start:end]), "(") && !strings.HasSuffix(strings.TrimSpace(line[start:end]), "{") && !strings.HasSuffix(strings.TrimSpace(line[start:end]), ",")
			hs = append(hs, head{sink: r.sink, name: name, start: start, end: end, bare: bare})
		}
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].start < hs[j].start })
	var out []head
	for _, h := range hs {
		if len(out) > 0 && h.start < out[len(out)-1].end {
			continue
		}
		out = append(out, h)
	}
	return out
}

// commentLine reports whether a line is a comment as a whole.
func commentLine(line string, l language) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") ||
		l.hashComments && strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "#{") && !strings.HasPrefix(t, "#[")
}

// inCode reports whether line[pos] is code: not inside a string or after a comment marker.
func inCode(line string, pos int, l language) bool {
	toks, _ := lex(line[:pos], 0, l)
	if len(toks) > 0 {
		last := toks[len(toks)-1]
		if last.kind == 's' && last.end >= pos && !closedString(last.text) {
			return false
		}
	}
	end := 0
	if len(toks) > 0 {
		end = toks[len(toks)-1].end
	}
	rest := strings.TrimSpace(line[end:pos])
	return rest == "" || !(strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, "#"))
}

func closedString(s string) bool {
	return len(s) >= 2 && s[len(s)-1] == s[0] && (len(s) < 3 || s[len(s)-2] != '\\')
}

// args reads a call's arguments from line[from:], adding findings, and returns where the call ends on this
// line. c.closed is set when the arguments end on this line.
func (s *Scanner) args(c *call, line string, from, number int, out *[]domain.Finding) int {
	toks, interpolated := lex(line, from, s.lang)
	end := len(line)
	var mine []token
loop:
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == 'p' {
			switch t.text {
			case "(", "[", "{":
				c.depth++
			case ")", "]", "}":
				c.depth--
			case ";":
				if c.depth <= 1 {
					c.depth = 0
				}
			}
			if c.depth <= 0 && !c.bare {
				// a chained call such as log.Info().Str("email", u.Email).Msg("") keeps going.
				if i+3 < len(toks) && toks[i+1].text == "." && toks[i+2].kind == 'i' && toks[i+3].text == "(" {
					mine = append(mine, t)
					c.depth = 1
					i += 3
					continue
				}
				c.closed = true
				end = t.end
				mine = append(mine, t)
				break loop
			}
		}
		mine = append(mine, t)
	}
	if c.bare {
		c.closed = true
	}
	if n := len(mine); n > 0 && mine[n-1].kind == 's' && !closedString(mine[n-1].text) {
		// a string that runs onto the next line is text, not arguments, so the call is not followed further.
		c.closed = true
	}
	for _, g := range interpolated {
		if len(g) > 0 && g[0].start < end {
			s.values(c, line, g, number, out, true)
		}
	}
	s.values(c, line, mine, number, out, false)
	return end
}

// values reports the expressions among the tokens that hold personal data.
func (s *Scanner) values(c *call, line string, toks []token, number int, out *[]domain.Finding, interpolated bool) {
	var wrappers []string
	depth := 0
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.kind {
		case 'p':
			switch t.text {
			case "(", "[", "{":
				depth++
				wrappers = append(wrappers, "")
			case ")", "]", "}":
				depth--
				if len(wrappers) > 0 {
					wrappers = wrappers[:len(wrappers)-1]
				}
			case ",":
				if depth == 0 {
					c.first = false
				}
			}
		case 's':
			if !interpolated && keyLike(toks, i, c.first && depth == 0) {
				name := strings.Trim(t.text, "\"'`")
				if v, ok := field(nil, name); ok {
					s.report(c, line, number, t.start, t.end, v, out)
				}
			}
			if depth == 0 {
				c.first = false
			}
		case 'i':
			p := readPath(toks, i)
			i = p.next - 1
			if p.call {
				if p.next < len(toks) && toks[p.next].text == "(" {
					wrappers = append(wrappers, strings.ToLower(p.segs[len(p.segs)-1]))
					depth++
					i = p.next
				}
				continue
			}
			if p.compared || negated(toks, p.first) || wrapped(wrappers) {
				continue
			}
			if p.key {
				if v, ok := field(nil, clean(p.segs[len(p.segs)-1])); ok && len(p.segs) == 1 {
					s.report(c, line, number, toks[p.first].start, toks[p.next-1].end, v, out)
				}
				continue
			}
			if v, ok := s.judge(p.segs); ok {
				s.report(c, line, number, toks[p.first].start, toks[p.next-1].end, v, out)
			} else if b, ok := s.through(p.segs[0], number, t.start); ok && len(p.segs) == 1 {
				v := b.v
				v.via = b.reason()
				s.report(c, line, number, toks[p.first].start, toks[p.next-1].end, v, out)
			}
		}
	}
}

func (s *Scanner) report(c *call, line string, number, start, end int, v verdict, out *[]domain.Finding) {
	key := string(v.category) + "/" + string(v.special) + "/" + v.kind
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	reasons := []string{v.reason}
	if v.via != "" {
		reasons = append(reasons, v.via)
	}
	reasons = append(reasons, fmt.Sprintf("passed to %s, %s", c.head, sinkWords[c.sink]))
	expr := strings.Join(strings.Fields(line[start:end]), " ")
	if len(expr) > 80 {
		expr = expr[:77] + "..."
	}
	*out = append(*out, domain.Finding{
		Kind:       domain.KindSink,
		Category:   v.category,
		Special:    v.special,
		Sink:       c.sink,
		Detector:   Detector,
		Label:      Labels[c.sink],
		Country:    domain.CrossBorder,
		Location:   domain.Location{Path: s.path, Line: number, Column: utf8.RuneCountInString(line[:start]) + 1},
		Masked:     expr,
		Confidence: v.confidence,
		Reasons:    reasons,
	})
}

// keyLike reports whether the string at toks[i] is a key naming the value next to it: "email": or 'email' =>
// in a map, or "email", in a list of key-value pairs that is not the call's first argument (the message).
func keyLike(toks []token, i int, first bool) bool {
	name := strings.Trim(toks[i].text, "\"'`")
	if name == "" || len(name) > 40 || strings.ContainsAny(name, " %{}$#\\/") {
		return false
	}
	if i+1 >= len(toks) {
		return false
	}
	switch toks[i+1].text {
	case ":", "=>":
		return true
	case ",":
		return !first && i+2 < len(toks) && toks[i+2].kind != 's'
	}
	return false
}

// negated reports whether the expression starting at toks[i] is negated, so only a boolean is passed.
func negated(toks []token, i int) bool {
	return i > 0 && toks[i-1].text == "!"
}

// safeWrappers are calls that pass on something derived from a value, not the value: its length, a hash or a
// masked copy.
var safeWrappers = []string{"mask", "redact", "hash", "sha", "md5", "anonymi", "obfuscat", "scrub", "sanitiz", "encrypt", "digest", "pseudonym", "tokeniz", "censor", "truncat"}

var safeExact = map[string]bool{"len": true, "count": true, "strlen": true, "mb_strlen": true, "sizeof": true, "isset": true, "empty": true, "bool": true, "size": true, "typeof": true, "type": true, "isinstance": true, "is_null": true, "length": true, "boolean": true, "exists": true, "hasattr": true, "getclass": true, "get_class": true, "class": true}

func wrapped(wrappers []string) bool {
	for _, w := range wrappers {
		if safeExact[w] {
			return true
		}
		for _, s := range safeWrappers {
			if strings.Contains(w, s) {
				return true
			}
		}
	}
	return false
}
