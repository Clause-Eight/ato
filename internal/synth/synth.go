// Package synth generates synthetic identifiers for tests and benchmarks. Every value is made up here and
// belongs to nobody. Check digits are computed independently of the detectors, so a validator is always
// tested against a second implementation of its rule. It imports nothing from this module.
package synth

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// Generator returns one valid value, written the way it usually appears in text.
type Generator func(r *rand.Rand) string

var generators = map[string]Generator{}

// Register adds the generator for a detector. Each detector file's generator lives in a file of the same country.
func Register(detector string, g Generator) {
	if _, dup := generators[detector]; dup {
		panic("synth: duplicate generator for " + detector)
	}
	generators[detector] = g
}

// For returns the generator for a detector.
func For(detector string) (Generator, bool) {
	g, ok := generators[detector]
	return g, ok
}

// Names lists the detectors that have a generator, sorted.
func Names() []string {
	out := make([]string, 0, len(generators))
	for n := range generators {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Digits returns n random digits.
func Digits(r *rand.Rand, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(byte('0' + r.Intn(10)))
	}
	return b.String()
}

// Pad writes n with leading zeros to width digits.
func Pad(n, width int) string { return fmt.Sprintf("%0*d", width, n) }

// Letters returns n random capital letters from set (A-Z when set is empty).
func Letters(r *rand.Rand, n int, set string) string {
	if set == "" {
		set = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(set[r.Intn(len(set))])
	}
	return b.String()
}

// Date returns a random valid date between 1930 and 2019 (days 1-28 keep every month valid).
func Date(r *rand.Rand) (year, month, day int) {
	return 1930 + r.Intn(90), 1 + r.Intn(12), 1 + r.Intn(28)
}

// WeightedSum multiplies each digit of s by its weight and adds them up. Non-digits panic: callers pass digits.
func WeightedSum(s string, weights []int) int {
	sum := 0
	for i, w := range weights {
		c := s[i]
		if c < '0' || c > '9' {
			panic("synth: weighted sum over a non-digit")
		}
		sum += int(c-'0') * w
	}
	return sum
}

// LuhnDigit is the digit that makes body+digit pass the Luhn check.
func LuhnDigit(body string) int {
	sum := 0
	for i := len(body) - 1; i >= 0; i-- {
		n := int(body[i] - '0')
		if (len(body)-1-i)%2 == 0 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return (10 - sum%10) % 10
}

// Mod1110Digit is the ISO 7064 MOD 11,10 check digit for body.
func Mod1110Digit(body string) int {
	product := 10
	for _, c := range body {
		sum := (int(c-'0') + product) % 10
		if sum == 0 {
			sum = 10
		}
		product = sum * 2 % 11
	}
	return (11 - product) % 10
}

// Mod97 is the remainder of a long decimal string divided by 97.
func Mod97(digits string) int {
	rem := 0
	for _, c := range digits {
		rem = (rem*10 + int(c-'0')) % 97
	}
	return rem
}

// LettersAsNumbers turns A-Z into 10-35, as ISO 13616 and ISO 7064 do.
func LettersAsNumbers(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			fmt.Fprint(&b, int(c-'A')+10)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func sortStrings(s []string) { sort.Strings(s) }
