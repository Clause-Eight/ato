// Package textfold normalises text for matching words in any EU language. It imports nothing from this module.
package textfold

import (
	"strings"
	"unicode"
)

// Fold prepares text for keyword matching: lower case, Latin diacritics and Greek accents removed, final sigma
// made ordinary, and underscores and dashes turned into spaces. Cyrillic is only lower-cased.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.TrimSpace(s) {
		r = unicode.ToLower(r)
		if r == '_' || r == '-' {
			b.WriteByte(' ')
			continue
		}
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		if f, ok := folds[r]; ok {
			b.WriteString(f)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// folds covers the letters of the EU's Latin and Greek alphabets that carry marks.
var folds = func() map[rune]string {
	m := map[rune]string{'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o", 'đ': "d", 'ł': "l", 'ħ': "h", 'ı': "i", 'ς': "σ"}
	groups := map[string]string{
		"a": "àáâãäåāăą", "c": "çćčċ", "d": "ď", "e": "èéêëēėęěĕ", "g": "ğġģ", "i": "ìíîïīįĭ", "k": "ķ",
		"l": "ĺļľ", "n": "ñńņň", "o": "òóôõöōőŏ", "r": "ŕř", "s": "śšşș", "t": "ťţț", "u": "ùúûüūůűųŭ",
		"y": "ýÿ", "z": "źżž",
		"α": "ά", "ε": "έ", "η": "ή", "ι": "ίϊΐ", "ο": "ό", "υ": "ύϋΰ", "ω": "ώ",
	}
	for base, marked := range groups {
		for _, r := range marked {
			m[r] = base
		}
	}
	return m
}()
