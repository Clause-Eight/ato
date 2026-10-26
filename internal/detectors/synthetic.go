package detectors

import (
	"crypto/sha256"
	"strings"
	"unicode"
)

// SyntheticBits is how many leading zero bits of a value's mark hash make it a synthetic value. `pii-eu fix`
// only writes values that carry the mark, so a later scan recognises them without a list of what was written.
// A real value carries the mark by chance about once in 65,536; it is then reported at low confidence as a
// sample, not dropped.
const SyntheticBits = 16

const syntheticSalt = "pii-eu synthetic v1\x00"

// Synthetic reports whether a value, as written in text, carries the mark of a synthetic value. Only letters
// and digits count, in upper case, so separators and case do not change the answer.
func Synthetic(text string) bool {
	c := Canonical(text)
	if c == "" {
		return false
	}
	sum := sha256.Sum256([]byte(syntheticSalt + c))
	for i := 0; i < SyntheticBits/8; i++ {
		if sum[i] != 0 {
			return false
		}
	}
	rest := SyntheticBits % 8
	return rest == 0 || sum[SyntheticBits/8]>>(8-rest) == 0
}

// Canonical keeps only the letters and digits of a value, in upper case.
func Canonical(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}
