package detectors

import "testing"

func TestRomanianIDCardRules(t *testing.T) {
	d, _ := Get("ro-id-card")
	if !d.NeedsKeyword {
		t.Fatalf("the series and number have no check digit: a keyword must be required")
	}
	cases := map[string]bool{
		"RX000001":      true,
		"KX 000002":     true,
		"CJ nr. 00003":  false, // five digits
		"CJ nr. 000003": true,
		"XV-000004":     true,
		"QQ000005":      false, // not a county series
		"AB000006":      false,
		"RX0000077":     false, // seven digits
		"rx000008":      false,
	}
	for v, want := range cases {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("case %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
	for _, v := range []string{"RX000001", "RX 000001", "RX nr. 000001", "RX NR 000001"} {
		if got := d.Pattern.FindString(v); got != v {
			t.Errorf("pattern does not find a %d-character value whole", len(v))
		}
	}
	if got := d.Mask("RX000001"); got != "••••••••" {
		t.Errorf("mask must hide the whole value")
	}
	if !d.Needs.Admits("seria RX nr. 000001", 6) || d.Needs.Admits("serial rx000001", 6) {
		t.Errorf("prefilter does not look for two capitals before a digit")
	}
}
