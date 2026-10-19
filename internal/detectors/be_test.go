package detectors

import "testing"

func TestBENationalRegister(t *testing.T) {
	d, ok := Get("be-national-register")
	if !ok {
		t.Fatal("not registered")
	}
	cases := []struct {
		why   string
		value string
		want  bool
	}{
		{"born in the 1900s", "85120412374", true},
		{"printed with dots and a dash", "85.12.04-123.74", true},
		{"born from 2000, check over 2 and the nine digits", "05061734401", true},
		{"incomplete birth date", "70000041783", true},
		{"bis number, sex known", "85420412390", true},
		{"bis number, sex unknown, month and day unknown", "70240041757", true},
		{"wrong check digits", "85120412375", false},
		{"born in 2000, bis number with sex unknown", "00290112379", true},
		{"month 13", "85133212317", false},
		{"30 February", "85023012397", false},
		{"serial 000", "85120400074", false},
		{"too short", "8512041237", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.value)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.why, got, c.want)
		}
	}
	if got := d.Normalize("85.12.04-123.74"); got != "85120412374" {
		t.Errorf("normalize kept the separators")
	}
	if m := d.Mask("85120412374"); len([]rune(m)) != 11 || m == "85120412374" {
		t.Errorf("mask does not hide the value")
	}
	if !d.Formatted.MatchString("85.12.04-123.74") || d.Formatted.MatchString("85120412374") {
		t.Errorf("the usual format is YY.MM.DD-SSS.CC")
	}
}
