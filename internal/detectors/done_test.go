package detectors

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/synth"
)

// TestDefinitionOfDone holds every detector to the bar in CONTRIBUTING.md: a synthetic generator, values the
// pattern finds whole, a validator that accepts them, a mask that keeps the length, and a check digit that
// rejects a changed digit.
func TestDefinitionOfDone(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for _, d := range All() {
		if weakChecks[d.Name] != "" && !d.NeedsKeyword {
			t.Errorf("%s has a weak check and must need a keyword", d.Name)
		}
		gen, ok := synth.For(d.Name)
		if !ok {
			t.Errorf("%s has no generator in internal/synth", d.Name)
			continue
		}
		if d.Label == "" || d.Validation == "" || d.Country == "" || len(d.Keywords)+len(d.Concepts) == 0 {
			t.Errorf("%s: label, validation, country and keywords or concepts are required", d.Name)
		}
		rejected := 0
		const n = 300
		for i := 0; i < n; i++ {
			v := gen(r)
			if got := d.Pattern.FindString(v); got != v {
				t.Fatalf("%s: the pattern does not find generated value %d whole (found %d of %d characters)", d.Name, i, len(got), len(v))
			}
			norm := d.Normalize(v)
			if d.Validate != nil && !d.Validate(norm) {
				t.Fatalf("%s: generated value %d is rejected", d.Name, i)
			}
			if m := d.Mask(norm); len([]rune(m)) != len([]rune(norm)) || m == norm {
				t.Fatalf("%s: the mask must hide the value and keep its length", d.Name)
			}
			if d.Validate != nil && !d.Validate(changeDigit(norm)) {
				rejected++
			}
		}
		if strings.Contains(d.Validation, "check") && weakChecks[d.Name] == "" && rejected < n*85/100 {
			t.Errorf("%s: a changed digit is rejected only %d times in %d", d.Name, rejected, n)
		}
	}
}

// weakChecks are official check rules that let more than one changed digit in seven through; each detector
// listed needs a keyword nearby.
var weakChecks = map[string]string{
	"gb-utr": "HMRC's table repeats 2 and 1, so two remainders in eleven give the same check digit",
}

// changeDigit adds one to the last digit of s.
func changeDigit(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] >= '0' && s[i] <= '9' {
			return s[:i] + string('0'+(s[i]-'0'+1)%10) + s[i+1:]
		}
	}
	return s
}
