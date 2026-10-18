package detectors

import "testing"

func TestItalianFiscalCode(t *testing.T) {
	wantValidity(t, "it-fiscal-code", map[string]bool{
		"QWTZPL85T10A562P": true,
		"KXJVNB92E51F205S": true,  // woman, day 11 + 40
		"DFGHJK70A01Z404W": true,  // born abroad (Z)
		"QWTZPL85T10A56NE": true,  // omocodia, last digit replaced
		"QWTZPLURTMLARNSV": true,  // omocodia, all seven digits replaced
		"KXJVNB92B69F205F": true,  // 29 February
		"QWTZPL85T10A562Q": false, // wrong check letter
		"QWTZPL85T10A563P": false, // changed digit
		"KXJVNB92B30F205I": false, // 30 February
		"KXJVNB92E00F205O": false, // day 0
		"KXJVNB92E35F205D": false, // day 35
		"KXJVNB92F01F205W": false, // F is not a month letter
		"KXJVNB92E51N205A": false, // municipality letter N
		"QWTZPL85T10A562":  false, // 15 characters
		"QWTZPL85T10A56WE": false, // W is not an omocodia letter
	})
	wantPatternFinds(t, "it-fiscal-code", "codice_fiscale=QWTZPL85T10A56NE;", "QWTZPL85T10A56NE")
	d, _ := Get("it-fiscal-code")
	if d.Needs.Admits("const MAX_RETRIES = 16", 2) || !d.Needs.Admits("cf: QWTZPL85T10A562P", 7) {
		t.Error("the prefilter must look for 16 capitals and digits in a row")
	}
	if m := d.Mask("QWTZPL85T10A562P"); m != "••••••••••••••••" {
		t.Errorf("mask: %q", m)
	}
}
