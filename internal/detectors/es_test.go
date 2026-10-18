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
