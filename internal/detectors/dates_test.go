package detectors

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

// Every value in these tests is synthetic or a published test vector of the specification it comes from.

func detector(t *testing.T, name string) Detector {
	t.Helper()
	d, ok := Get(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	if d.OnByDefault {
		t.Errorf("%s must be off by default", name)
	}
	return d
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestBirthDate(t *testing.T) {
	d := detector(t, "birth-date")
	if !d.NeedsKeyword {
		t.Error("birth-date must need a keyword")
	}
	future := fmt.Sprintf("%d-01-01", time.Now().Year()+1)
	checkCases(t, "birth-date", map[string]bool{
		"1985-03-12":     true,
		"12.03.1985":     true,
		"12/03/1985":     true,
		"12-03-1985":     true,
		"2.3.1985":       true,
		"1985.03.12.":    true,
		"1985. 03. 12.":  true,
		"12. 3. 1985":    true,
		"12. 3. 1985.":   true,
		"29.02.2000":     true,
		"12 March 1985":  true,
		"12-Mar-1985":    true,
		"March 12, 1985": true,
		"29.02.1985":     false, // not a leap year
		"31.04.1985":     false,
		"12.13.1985":     false, // month 13
		"03/25/1985":     false, // month first
		"12.03-1985":     false, // mixed separators
		"1899-12-31":     false,
		future:           false,
		"12 Mars 1985":   false,
	})
	for _, s := range []string{"x 1985-03-12 y", "dob: 12. 3. 1985.", "born 12 March 1985,"} {
		if d.Pattern.FindString(s) == "" {
			t.Errorf("the pattern misses a date layout in case %q", s[:4])
		}
	}
	if m := d.Mask("1985-03-12"); m != "••••••••••" {
		t.Errorf("the date must be masked fully, got %q", m)
	}
}
