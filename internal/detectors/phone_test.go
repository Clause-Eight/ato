package detectors

import "testing"

func TestPhoneInternationalNumbersFollowEachPlan(t *testing.T) {
	cases := map[string]bool{
		"+40 712 345 678":     true,
		"0040712345678":       true,
		"+49 (0)30 1234 5678": true,
		"+49 151 2345 6789":   true,
		"+33 6 12 34 56 78":   true,
		"+33.1.23.45.67.89":   true,
		"+39 06 1234 5678":    true,
		"+39 312 345 6789":    true,
		"+34 612 345 678":     true,
		"+31 6 12345678":      true,
		"+351 912 345 678":    true,
		"+30 691 234 5678":    true,
		"+372 5123 4567":      true,
		"+357 99 123456":      true,
		"+353 87 123 4567":    true,
		"+356 7912 3456":      true,
		"+420 601 234 567":    true,
		"+421 901 234 567":    true,
		"+386 41 234 567":     true,
		"+370 612 34567":      true,
		"+371 2123 4567":      true,
		"+352 621 123 456":    true,
		"+46 70 123 45 67":    true,
		"+358 40 123 4567":    true,
		"+45 21 23 45 67":     true,
		"+43 664 123 4567":    true,
		"+32 470 12 34 56":    true,
		"+385 91 234 5678":    true,
		"+359 88 123 4567":    true,
		"+36 30 123 4567":     true,
		"+48 501 234 567":     true,
		"0712 345 678":        true,
		"0888 123 456":        true,
		"+4071234567":         false, // one digit short
		"+40812345678":        false, // no Romanian range starts with 8
		"+33 0 12 34 56 78":   false, // trunk 0 kept after the country code
		"+34 512 345 678":     false, // personal numbering, not mobile or geographic
		"+30 791 234 5678":    false, // wrong leading digit
		"+357 3 1234567":      false,
		"+371 31234567":       false,
		"+420 1234 5678":      false,
		"+31 69 1234567":      false,
		"+351 94 1234567":     false,
		"+49 15 1234":         false,
		"+44 20 7946 0000":    false, // not a member state
		"0612345678":          false, // national format outside RO and BG
	}
	for v, want := range cases {
		if got := validPhone(normalizePhone(v)); got != want {
			t.Errorf("phone case %q: got %v, want %v", v, got, want)
		}
	}
}

func TestPhoneNormalisation(t *testing.T) {
	for in, want := range map[string]string{
		"0049 (0)30-1234 5678": "+493012345678",
		"+40 (712) 345.678":    "+40712345678",
		"0712-345-678":         "0712345678",
	} {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalise %q: got %q, want %q", in, got, want)
		}
	}
	d, _ := Get("eu-phone")
	if m := d.Mask("+40712345678"); m != "••••••••••78" {
		t.Errorf("mask: %q", m)
	}
}

func TestPhonePatternFindsWholeNumbers(t *testing.T) {
	d, _ := Get("eu-phone")
	for _, v := range []string{"+49 (0)30 1234 5678", "+33 6 12 34 56 78", "0040-712-345-678", "+386 41 234 567"} {
		if got := d.Pattern.FindString("tel: " + v + " ok"); got != v {
			t.Errorf("found %d of %d characters", len(got), len(v))
		}
	}
	for _, line := range []string{"+44 20 7946 0000", "+1 202 555 0100", "+41 44 123 45 67"} {
		if d.Pattern.MatchString(line) {
			t.Errorf("a number outside the member states matched")
		}
	}
}
