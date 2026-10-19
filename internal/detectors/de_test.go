package detectors

import (
	"fmt"
	"testing"
)

// withMod1110 appends the ISO 7064 MOD 11,10 check digit, so a case breaks only the rule it is about.
func withMod1110(body string) string {
	for c := 0; c < 10; c++ {
		if v := body + fmt.Sprint(c); mod11_10(v) {
			return v
		}
	}
	panic("no check digit")
}

func TestDETaxID(t *testing.T) {
	d, _ := Get("de-tax-id")
	cases := []struct {
		name, in string
		want     bool
	}{
		{"one digit twice", "18976083252", true},
		{"written in groups", "47 065 932 716", true},
		{"one digit three times, two adjacent", withMod1110("1121345678"), true},
		{"one digit three times, none adjacent", withMod1110("1213145678"), true},
		{"wrong check digit", "18976083253", false},
		{"leading zero", withMod1110("0897608325"), false},
		{"every digit once", withMod1110("1234567890"), false},
		{"two digits twice", withMod1110("1122345678"), false},
		{"one digit four times", withMod1110("1211213456"), false},
		{"three adjacent equal digits", withMod1110("1112345678"), false},
		{"too short", "1897608325", false},
		{"too long", "189760832520", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	for s := range d.Samples {
		if !d.Validate(s) {
			t.Error("a published sample does not validate")
		}
	}
	if got := d.Normalize("47 065 932 716"); len(got) != 11 {
		t.Errorf("normalised length %d, want 11", len(got))
	}
	if m := d.Mask("18976083252"); m != "•••••••••••" {
		t.Errorf("mask %q", m)
	}
}
