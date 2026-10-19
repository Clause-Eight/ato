package detectors

import "testing"

func TestIEPPSN(t *testing.T) {
	d, ok := Get("ie-ppsn")
	if !ok {
		t.Fatal("not registered")
	}
	cases := []struct {
		why   string
		value string
		want  bool
	}{
		{"old format, one letter", "7654321G", true},
		{"new format, second letter A", "2468024EA", true},
		{"new format, second letter B", "1357913SB", true},
		{"old format with W, which does not count", "8080808VW", true},
		{"wrong check letter", "7654321H", false},
		{"second letter ignored when it counts", "2468024SA", false},
		{"unknown second letter", "2468024EZ", false},
		{"six digits", "765432G", false},
		{"no letter", "7654321", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.value)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.why, got, c.want)
		}
	}
	if m := d.Mask("2468024EA"); len([]rune(m)) != 9 || m == "2468024EA" {
		t.Errorf("mask does not hide the value")
	}
	if !d.Needs.Admits("pps: 2468024EA", 7) || d.Needs.Admits("order 2468024 ea", 7) {
		t.Errorf("the prefilter must need a digit followed by a capital letter")
	}
}
