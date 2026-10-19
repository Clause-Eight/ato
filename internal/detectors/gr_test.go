package detectors

import "testing"

// Every value in this file is synthetic, with check digits computed by hand outside this package.

func TestGreekAMKA(t *testing.T) {
	d, _ := Get("gr-amka")
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid", "14078503829", true},
		{"31 February", "31028503824", false},
		{"wrong Luhn digit", "14078503820", false},
		{"29 February in a year that is never a leap year", "29020104476", false},
		{"month 13", "14138503820", false},
		{"day 0", "00078503829", false},
		{"too short", "1407850382", false},
		{"too long", "140785038290", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if m := d.Mask("14078503829"); len([]rune(m)) != 11 || m == "14078503829" {
		t.Error("the mask must hide the value and keep its length")
	}
}

func TestGreekAFM(t *testing.T) {
	d, _ := Get("gr-afm")
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid, leading zero", "073642150", true},
		{"valid", "158293049", true},
		{"wrong check digit", "158293040", false},
		{"all zeros", "000000000", false},
		{"too short", "15829304", false},
		{"letters", "15829304A", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
