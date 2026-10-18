package detectors

import "testing"

func TestSpanishDNI(t *testing.T) {
	wantValidity(t, "es-dni", map[string]bool{
		"48151623L":  true,
		"48151623-L": true,
		"48151623 L": true,
		"05000000F":  true,
		"10293847J":  true,
		"48151623M":  false, // wrong letter
		"48151624L":  false, // changed digit
		"4815162L":   false, // 7 digits
		"48151623I":  false, // I is never a check letter
		"X1234567L":  false, // a NIE is not a DNI
	})
	wantPatternFinds(t, "es-dni", "dni: 48151623-L.", "48151623-L")
	d, _ := Get("es-dni")
	if got := d.Normalize("48151623-L"); got != "48151623L" {
		t.Errorf("normalised to %d characters", len(got))
	}
}

func TestSpanishNIE(t *testing.T) {
	wantValidity(t, "es-nie", map[string]bool{
		"X1234567L":   true,
		"X-1234567-L": true,
		"Y7654321G":   true,
		"Z2468024Z":   true,
		"X1234567M":   false, // wrong letter
		"Y1234567L":   false, // Y stands for 1, not 0
		"W1234567L":   false, // unknown prefix
		"X123456L":    false, // 6 digits
		"48151623L":   false, // a DNI is not a NIE
	})
	wantPatternFinds(t, "es-nie", "nie=Y7654321G", "Y7654321G")
	d, _ := Get("es-nie")
	if d.Needs.Admits("count: 12345678", 8) {
		t.Error("a line without X, Y or Z cannot hold a NIE")
	}
}

func TestSpanishNSS(t *testing.T) {
	wantValidity(t, "es-nss", map[string]bool{
		"298765432101":     true,
		"29 87654321 01":   true,
		"15/20406080/47":   true,
		"085123456709":     true,
		"661000000087":     true,  // abroad (66)
		"410123456760":     true,  // sequence below 10,000,000
		"280000000091":     true,  // sequence 0
		"410123456771":     false, // below 10,000,000 the plain concatenation is not the rule
		"298765432102":     false, // wrong check digits
		"541234567812":     false, // province 54
		"001234567803":     false, // province 00
		"29876543210":      false, // 11 digits
		"2987654321011":    false, // 13 digits
		"29 87654321 0":    false,
		"29-87654321-0122": false,
	})
	wantPatternFinds(t, "es-nss", "naf: 29/87654321/01", "29/87654321/01")
	d, _ := Get("es-nss")
	if !d.Formatted.MatchString("29/87654321/01") || d.Formatted.MatchString("298765432101") {
		t.Error("only the separated form is the usual format")
	}
}
