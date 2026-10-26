package detectors

import "testing"

// The values below are made up so that they pass (or fail) the published rules; they belong to nobody.

func TestPolishIDCardRules(t *testing.T) {
	d, _ := Get("pl-id-card")
	cases := map[string]bool{
		"ABA300000":  true, // the worked example of the check-digit rule
		"XYZ712345":  true,
		"KLM900042":  true,
		"ABA400000":  false, // wrong check digit
		"XYZ812345":  false,
		"AAA000000":  false, // series AAA is not issued
		"AB1300000":  false, // a digit in the series
		"ABA30000":   false, // too short
		"ABA3000000": false, // too long
		"aba300000":  false, // lower case
	}
	for v, want := range cases {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("case %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
	if got := d.Normalize("ABA 300000"); got != "ABA300000" {
		t.Errorf("normalize kept the space")
	}
	if !d.Needs.Admits("dowód: ABA 300000", 6) || d.Needs.Admits("let x = abc300000", 6) || d.Needs.Admits("ABCD300000", 6) {
		t.Errorf("prefilter does not look for exactly three capitals before a digit")
	}
}

func TestPolishREGONRules(t *testing.T) {
	d, _ := Get("pl-regon")
	if !d.NeedsKeyword || d.Base > 1 {
		t.Fatalf("REGON names businesses: it must be low and need a keyword")
	}
	cases := map[string]bool{
		"123456785":      true,
		"123456786":      false, // wrong check digit
		"12345678500010": true,
		"12345678500011": false, // wrong local-unit check digit
		"12345678600010": false, // the first nine digits are not a valid REGON
		"1234567850001":  false, // thirteen digits
		"12345678":       false,
	}
	for v, want := range cases {
		if got := d.Validate(v); got != want {
			t.Errorf("case %d digits, valid=%v: got %v", len(v), want, got)
		}
	}
}

func TestPolishNIPRules(t *testing.T) {
	d, _ := Get("pl-nip")
	cases := map[string]bool{
		"2234567895":    true, // the worked example in the OECD TIN sheet
		"223-456-78-95": true,
		"223-45-67-895": true,
		"2234567896":    false, // wrong check digit
		"1234567080":    false, // remainder 10 is never issued
		"1234567089":    false,
		"0000000000":    false,
		"223456789":     false, // too short
	}
	for v, want := range cases {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("case %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
	for _, v := range []string{"223-456-78-95", "223-45-67-895", "2234567895"} {
		if d.Pattern.FindString(v) != v {
			t.Errorf("a %d-character form is not found whole", len(v))
		}
	}
}
