package sinks

import (
	"fmt"
	"regexp"
	"strings"
)

// window is how many lines after its assignment a local variable is followed.
const window = 30

// binding is a local variable assigned a value that holds personal data: v = user.email. A sink call that is
// later passed the bare name v is reported as passing user.email.
type binding struct {
	name string
	v    verdict
	line int
	end  int
}

// declarers start a declaration whose next name is the variable: const v, let v, var v, val v.
var declarers = map[string]bool{"const": true, "let": true, "var": true, "val": true, "my": true, "local": true}

// modifiersDecl may stand in front of a declaration: final String v.
var modifiersDecl = map[string]bool{"final": true, "auto": true, "readonly": true, "static": true, "private": true, "public": true, "protected": true, "lateinit": true}

// functionStart recognises a line that starts a function or method, where local variables go out of scope.
var functionStart = regexp.MustCompile(`^\s*(?:(?:export|async|public|private|protected|internal|static|final|abstract|override|suspend|open|inline|default|synchronized|operator)\s+)*(?:def|func|function|fun|sub)\b`)

// methodStart recognises a C-style method header such as void handle(User user) {.
var methodStart = regexp.MustCompile(`^\s*(?:[\w<>\[\],.?]+\s+)+\w+\s*\([^;=]*\)\s*(?:throws\s+[\w.,\s]+)?\{?\s*$`)

var notMethod = map[string]bool{"if": true, "for": true, "foreach": true, "while": true, "switch": true, "catch": true, "return": true, "else": true, "new": true, "throw": true, "do": true, "try": true, "case": true, "elif": true, "elsif": true, "when": true, "unless": true, "until": true, "await": true, "yield": true, "go": true, "defer": true, "echo": true, "print": true, "puts": true}

// startsFunction reports whether a line starts a new function or method.
func startsFunction(line string) bool {
	if functionStart.MatchString(line) {
		return true
	}
	if !methodStart.MatchString(line) {
		return false
	}
	f := strings.Fields(line)
	return len(f) > 0 && !notMethod[strings.ToLower(f[0])]
}

// assignments reads the statements of line[from:], forgetting variables that are reassigned and remembering
// those assigned a value that holds personal data.
func (s *Scanner) assignments(line string, from, number int) {
	if s.vars == nil {
		s.vars = map[string]binding{}
	}
	for name, b := range s.vars {
		if number-b.line > window {
			delete(s.vars, name)
		}
	}
	if from == 0 && startsFunction(line) {
		clear(s.vars)
		return
	}
	toks, _ := lex(line, from, s.lang)
	start := 0
	for i := 0; i <= len(toks); i++ {
		if i < len(toks) && toks[i].text != ";" {
			continue
		}
		if i > start {
			end := len(line)
			if i < len(toks) {
				end = toks[i].start
			}
			s.statement(toks[start:i], number, end)
		}
		start = i + 1
	}
}

// statement handles one statement: a plain assignment of one expression to one local name binds the name;
// any other write to a known name forgets it.
func (s *Scanner) statement(toks []token, number, end int) {
	s.forget(toks)
	eq := -1
	for i, t := range toks {
		if t.text == "=" {
			eq = i
			break
		}
	}
	if eq <= 0 {
		return
	}
	lhs := toks[:eq]
	if n := len(lhs); n > 1 && lhs[n-1].text == ":" && lhs[n-1].end == toks[eq].start {
		lhs = lhs[:n-1]
	}
	name, ok := target(lhs)
	if !ok {
		return
	}
	rhs := toks[eq+1:]
	if len(rhs) == 0 || rhs[0].kind != 'i' {
		return
	}
	p := readPath(rhs, 0)
	if p.call || p.next != len(rhs) {
		return
	}
	if v, ok := s.judge(p.segs); ok {
		s.vars[name] = binding{name: name, v: v, line: number, end: end}
	} else if b, ok := s.through(p.segs[0], number, rhs[0].start); ok && len(p.segs) == 1 {
		// b = a, where a already holds personal data: b holds it too, so a chain of copies is followed.
		s.vars[name] = binding{name: name, v: b.v, line: number, end: end}
	}
}

// forget drops the known names that a statement writes to: the left of an assignment (v = 1, v += 1, v ||= x)
// and names bound by a loop, a lambda or a catch (for v in, as v, v ->, |v|).
func (s *Scanner) forget(toks []token) {
	if len(s.vars) == 0 {
		return
	}
	eq := -1
	for i, t := range toks {
		if t.text == "=" {
			eq = i
			break
		}
	}
	loop := len(toks) > 0 && (toks[0].text == "for" || toks[0].text == "foreach")
	for i, t := range toks {
		if t.kind != 'i' {
			continue
		}
		if _, known := s.vars[t.text]; !known {
			continue
		}
		prev, next := "", ""
		if i > 0 {
			prev = toks[i-1].text
		}
		if i+1 < len(toks) {
			next = toks[i+1].text
		}
		if prev == "." || prev == "->" || prev == "?." || prev == "::" {
			continue
		}
		if i < eq || loop || prev == "as" || prev == "|" || next == "|" || next == "->" || next == "=>" || next == "in" || next == "of" {
			delete(s.vars, t.text)
		}
	}
}

// target returns the local name on the left of an assignment: v, $v, const v, val v: String, final String v,
// List<String> v. Fields (this.v), indexes (m[k]) and several names (a, b) are not local names.
func target(lhs []token) (string, bool) {
	i := 0
	for i < len(lhs) && lhs[i].kind == 'i' && modifiersDecl[lhs[i].text] {
		i++
	}
	if i < len(lhs) && lhs[i].kind == 'i' && declarers[lhs[i].text] {
		i++
		if i < len(lhs) && lhs[i].kind == 'i' {
			return name(lhs[i].text)
		}
		return "", false
	}
	rest := lhs[i:]
	angle := 0
	for j, t := range rest {
		switch {
		case t.kind == 'i':
		case t.text == "<":
			angle++
		case t.text == ">":
			angle--
		case t.text == "," && angle > 0:
		case t.text == "?":
		case t.text == "[" && j+1 < len(rest) && rest[j+1].text == "]":
		case t.text == "]" && j > 0 && rest[j-1].text == "[":
		case t.text == ":":
			if j > 0 && rest[j-1].kind == 'i' && angle == 0 {
				return name(rest[j-1].text)
			}
			return "", false
		default:
			return "", false
		}
	}
	if len(rest) == 0 || rest[len(rest)-1].kind != 'i' || angle != 0 {
		return "", false
	}
	return name(rest[len(rest)-1].text)
}

func name(s string) (string, bool) {
	if s == "" || s[0] == '@' || keywords[strings.ToLower(clean(s))] {
		return "", false
	}
	return s, true
}

// through returns the binding of a bare name passed to a sink at line[start], if it holds personal data.
func (s *Scanner) through(name string, number, start int) (binding, bool) {
	if s.lang.name == "kotlin" {
		name = strings.TrimPrefix(name, "$")
	}
	b, ok := s.vars[name]
	if !ok || number-b.line > window || b.line == number && b.end > start {
		return binding{}, false
	}
	return b, true
}

func (b binding) reason() string {
	return fmt.Sprintf("through the variable %s assigned on line %d", b.name, b.line)
}
