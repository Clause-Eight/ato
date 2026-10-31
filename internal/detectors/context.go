package detectors

import (
	"strings"
	"unicode"
)

// keyTokens splits the key written just before a value into folded words: `"card_cvv": "`, `cvc=`,
// `'securityCode' => '`, `<cvv>`, `Security code: `. Quotes, brackets, colons and equals signs between the key
// and the value are skipped; the key is the run of letters, digits, separators and spaces before them.
func keyTokens(before string) []string {
	s := strings.TrimRight(before, " \t\r\n\"'`:=>[({<")
	i := len(s)
	for i > 0 && len(s)-i < 48 {
		c := s[i-1]
		if !(isASCIIAlnum(c) || c == '_' || c == '-' || c == '.' || c == ' ' || c >= 0x80) {
			break
		}
		i--
	}
	return nameTokens(s[i:])
}

// nameTokens splits a name into folded words at separators and camelCase humps.
func nameTokens(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, Fold(string(cur)))
			cur = cur[:0]
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && i > 0 && unicode.IsLower(runes[i-1]):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

// labelled reports whether the last one to four words of a key, written together, are one of labels.
func labelled(tokens []string, labels map[string]bool) bool {
	joined := ""
	for k := 1; k <= 4 && k <= len(tokens); k++ {
		joined = tokens[len(tokens)-k] + joined
		if labels[joined] {
			return true
		}
	}
	return false
}

// keyOrColumn reports whether the key just before a value, or else its column, is one of labels.
func keyOrColumn(a Around, labels map[string]bool) bool {
	return labelled(keyTokens(a.Before), labels) || columnLabel(a, labels)
}

// columnLabel reports whether a value fills its cell and the cell's column is one of labels.
func columnLabel(a Around, labels map[string]bool) bool {
	return a.Column != "" && strings.Trim(a.Before, " \t\"'") == "" && labelled(nameTokens(a.Column), labels)
}

// containsFold reports whether s holds one of words, ignoring ASCII case. Words are lower case.
func containsFold(s string, words []string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i] | 0x20
		for _, w := range words {
			if c == w[0] && len(s)-i >= len(w) && strings.EqualFold(s[i:i+len(w)], w) {
				return true
			}
		}
	}
	return false
}

func isASCIIAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
