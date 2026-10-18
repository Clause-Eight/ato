package detectors

import (
	"fmt"
	"testing"
)

// withPensionCheck appends the computed check digit, so a case breaks only the rule it is about.
func withPensionCheck(body string) string {
	c, ok := dePensionCheck(body)
	if !ok {
		panic("bad body")
	}
	return body + fmt.Sprint(c)
}

func TestDEPension(t *testing.T) {
	d, _ := Get("de-pension")
	cases := []struct {
		name, in string
		want     bool
	}{
		{"plain", "16120478F969", true},
		{"written in groups", "09 080771 U 648", true},
		{"Bund area number", withPensionCheck("53150380M01"), true},
		{"Knappschaft-Bahn-See area number", withPensionCheck("89150380M51"), true},
		{"29 February", withPensionCheck("12290204K12"), true},
		{"wrong check digit", "16120478F968", false},
		{"unknown area number", withPensionCheck("01150380M01"), false},
		{"area number 22 is not issued", withPensionCheck("22150380M01"), false},
		{"day 32", withPensionCheck("12320180M01"), false},
		{"month 13", withPensionCheck("12151380M01"), false},
		{"30 February", withPensionCheck("12300280M01"), false},
		{"lower-case letter", "16120478f969", false},
		{"digit instead of letter", "161204781969", false},
		{"too short", "16120478F96", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := d.Normalize("09 080771 U 648"); got != "09080771U648" {
		t.Errorf("normalised to %d characters", len(got))
	}
	if m := d.Mask("16120478F969"); m != "••••••••••••" {
		t.Errorf("mask %q", m)
	}
}
