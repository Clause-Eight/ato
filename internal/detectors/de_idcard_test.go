package detectors

import (
	"fmt"
	"testing"
)

func withICAO(body string) string { return body + fmt.Sprint(icaoCheckDigit(body)) }

func TestDEIDCard(t *testing.T) {
	d, _ := Get("de-id-card")
	cases := []struct {
		name, in string
		want     bool
	}{
		{"letters and digits", "TZ4C4F7ZP2", true},
		{"letters only before the check digit", withICAO("PNRVKXJFW"), true},
		{"wrong check digit", "TZ4C4F7ZP3", false},
		{"passport first letter", withICAO("CZ4C4F7ZP"), false},
		{"vowel inside", withICAO("TZ4A4F7ZP"), false},
		{"excluded consonant inside", withICAO("TZ4B4F7ZP"), false},
		{"lower case", "tz4c4f7zp2", false},
		{"letter as check digit", "TZ4C4F7ZPX", false},
		{"without check digit", "TZ4C4F7ZP", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if icaoCheckDigit("L898902C3") != 6 {
		t.Error("the ICAO 9303 worked example does not give 6")
	}
	for _, line := range []string{"ausweis: TZ4C4F7ZP2", "IDD<<TZ4C4F7ZP2<<<<"} {
		if !d.Needs.Admits(line, 5) {
			t.Errorf("prefilter rejects a line with a number")
		}
	}
	for _, line := range []string{"timeout: 30s", "ATZ4C4F7ZP2", "Total 12"} {
		if d.Needs.Admits(line, 5) {
			t.Errorf("prefilter admits %q", line)
		}
	}
	if m := d.Mask("TZ4C4F7ZP2"); m != "••••••••••" {
		t.Errorf("mask %q", m)
	}
}
