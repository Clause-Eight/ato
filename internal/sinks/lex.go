package sinks

import (
	"strings"
)

// token is one piece of code in a call's arguments. Kind is 'i' for an identifier, 's' for a string literal,
// 'n' for a number and 'p' for punctuation. Start and End are byte offsets in the line.
type token struct {
	kind       byte
	text       string
	start, end int
}

// lex splits line[from:] into tokens, stopping at a comment. Interpolated code inside strings ("#{user.email}",
// `${user.email}`, f"{user.email}", "$user->email") is returned as separate groups, since it is code passed to
// the call even though it sits in a string.
func lex(line string, from int, lang language) (tokens []token, interpolated [][]token) {
	i := from
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case lang.slashComments && strings.HasPrefix(line[i:], "//"),
			lang.hashComments && c == '#' && !strings.HasPrefix(line[i:], "#{") && !strings.HasPrefix(line[i:], "#["):
			return tokens, interpolated
		case c == '"' || c == '\'' || c == '`':
			fstring := false
			if lang.name == "python" && len(tokens) > 0 {
				prev := tokens[len(tokens)-1]
				if prev.kind == 'i' && prev.end == i && strings.ContainsAny(prev.text, "fF") && len(prev.text) <= 2 {
					fstring = true
					tokens = tokens[:len(tokens)-1]
				}
			}
			end, groups := readString(line, i, lang, fstring)
			for _, g := range groups {
				inner, more := lex(line[:g[1]], g[0], lang)
				interpolated = append(interpolated, inner)
				interpolated = append(interpolated, more...)
			}
			tokens = append(tokens, token{kind: 's', text: line[i:end], start: i, end: end})
			i = end
		case isIdentStart(c):
			j := i + 1
			for j < len(line) && isIdentPart(line[j]) {
				j++
			}
			tokens = append(tokens, token{kind: 'i', text: line[i:j], start: i, end: j})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(line) && (isIdentPart(line[j]) || line[j] == '.') {
				j++
			}
			tokens = append(tokens, token{kind: 'n', text: line[i:j], start: i, end: j})
			i = j
		default:
			n := 1
			for _, op := range []string{"?.", "->", "::", "=>", "==", "!=", "<=", ">=", "&&", "||"} {
				if strings.HasPrefix(line[i:], op) {
					n = 2
					break
				}
			}
			if n == 1 && c >= 0x80 {
				for n < 4 && i+n < len(line) && line[i+n]&0xC0 == 0x80 {
					n++
				}
			}
			tokens = append(tokens, token{kind: 'p', text: line[i : i+n], start: i, end: i + n})
			i += n
		}
	}
	return tokens, interpolated
}

// readString returns where the string starting at line[start] ends, and the byte ranges of code interpolated
// into it. An unterminated string ends at the end of the line.
func readString(line string, start int, lang language, fstring bool) (int, [][2]int) {
	quote := line[start]
	var groups [][2]int
	interpolates := fstring ||
		quote == '`' && lang.name == "js" ||
		quote == '"' && (lang.name == "ruby" || lang.name == "php" || lang.name == "kotlin")
	i := start + 1
	for i < len(line) {
		c := line[i]
		switch {
		case c == '\\':
			i += 2
			continue
		case c == quote:
			return i + 1, groups
		case interpolates:
			if g, next, ok := interpolation(line, i, lang, fstring); ok {
				groups = append(groups, g)
				i = next
				continue
			}
		}
		i++
	}
	return len(line), groups
}

// interpolation recognises the start of interpolated code at line[i] and returns its range and where the
// string continues after it.
func interpolation(line string, i int, lang language, fstring bool) ([2]int, int, bool) {
	rest := line[i:]
	open := 0
	switch {
	case fstring && strings.HasPrefix(rest, "{{"):
		return [2]int{}, 0, false
	case fstring && rest[0] == '{':
		open = 1
	case (lang.name == "js" || lang.name == "kotlin") && strings.HasPrefix(rest, "${"):
		open = 2
	case lang.name == "ruby" && strings.HasPrefix(rest, "#{"):
		open = 2
	case lang.name == "php" && strings.HasPrefix(rest, "{$"):
		open = 1
	case (lang.name == "php" || lang.name == "kotlin") && rest[0] == '$' && len(rest) > 1 && isIdentStart(rest[1]):
		j := i + 1
		for j < len(line) && isIdentPart(line[j]) {
			j++
		}
		for lang.name == "php" && strings.HasPrefix(line[j:], "->") && j+2 < len(line) && isIdentStart(line[j+2]) {
			j += 2
			for j < len(line) && isIdentPart(line[j]) {
				j++
			}
		}
		return [2]int{i, j}, j, true
	default:
		return [2]int{}, 0, false
	}
	depth := 1
	for j := i + open; j < len(line); j++ {
		switch line[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end := j
				if fstring {
					if k := strings.IndexAny(line[i+open:j], "!:="); k >= 0 {
						end = i + open + k
					}
				}
				return [2]int{i + open, end}, j + 1, true
			}
		}
	}
	return [2]int{}, 0, false
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c == '@' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdentPart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
