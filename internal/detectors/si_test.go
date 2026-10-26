package detectors

import "testing"

// Every value in this file is synthetic, with check digits computed by hand outside this package.

func TestSlovenianEMSO(t *testing.T) {
	d, _ := Get("si-emso")
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"born in the 1900s, register 50", "1503980502312", true},
		{"29 February 2000", "2902000507319", true},
		{"register 57", "1205975572107", true},
		{"wrong check digit", "1503980502313", false},
		{"29 February 2001", "2902001507312", false},
		{"year starting with 8", "1503891502317", false},
		{"year in the future", "1503099502318", false},
		{"register outside Slovenia", "1205975652100", false},
		{"remainder 1 is never issued", "0306985505120", false},
		{"too short", "150398050231", false},
		{"too long", "15039805023120", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSITaxNumber(t *testing.T) {
	d, _ := Get("si-tax-number")
	cases := map[string]bool{
		"97803570": true,
		"33975825": true,
		"60704144": true,
		"97803579": false, // wrong check digit
		"81297920": false, // remainder 0 is never issued
		"81297929": false,
		"01234567": false, // first digit 0
		"9780357":  false, // too short
	}
	for v, want := range cases {
		if got := d.Validate(v); got != want {
			t.Errorf("case %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
}
