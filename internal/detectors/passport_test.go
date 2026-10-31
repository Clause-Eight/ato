package detectors

import "testing"

func TestPassportLayouts(t *testing.T) {
	d, _ := Get("passport")
	if !d.NeedsKeyword {
		t.Fatalf("passport numbers have no check digit: a keyword must be required")
	}
	checkCases(t, "passport", map[string]bool{
		"P 1234567":   true,  // AT with its space
		"K1234567":    true,  // AT, EE, SK
		"N123456":     true,  // PT
		"E12345678":   true,  // CY, eight digits
		"EH123456":    true,  // BE
		"XP1234567":   true,  // FI, GR, PL
		"PB1234567":   true,  // SI
		"YA1234567":   true,  // IT
		"PAA123456":   true,  // ES
		"12AB34567":   true,  // FR
		"A12B3C4D":    true,  // LT
		"123456789":   true,  // nine digits
		"12345678":    true,  // eight digits
		"1234567":     true,  // MT
		"X1234567":    true,  // CH 2010
		"NPKR4F7J3":   true,  // NL
		"CFGH4JK7Z":   true,  // DE without its check digit
		"ABCDEFGH":    false, // no digit
		"123456":      false, // too short
		"12345678901": false, // too long
		"AB12345":     false, // two letters and five digits fit no layout
		"1234A5678":   false, // a letter in fifth place fits no layout
		"AOCD12345":   false, // four letters fit only the Dutch layout, which never uses O
		"LFGH4JK7Z1":  false, // ten characters but not a German passport serial
	})
	if de := withICAO("CFGH4JK7Z"); !d.Validate(de) || d.Validate(changeDigit(de)) {
		t.Errorf("the German check digit is not verified")
	}
	for _, line := range []string{"passport no: X1234567", "Reisepass: C01X00T47", "αριθμός διαβατηρίου: AB1234567", "номер на паспорт 123456789"} {
		if !d.Needs.Admits(line, 7) {
			t.Errorf("prefilter rejects a line naming a passport")
		}
	}
	if d.Needs.Admits("order_id: 123456789", 9) {
		t.Errorf("prefilter admits a line that does not name a passport")
	}
	if m := d.Mask("X1234567"); m != "••••••••" {
		t.Errorf("mask must hide the whole value")
	}
}
