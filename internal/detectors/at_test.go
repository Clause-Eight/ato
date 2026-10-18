package detectors

import "testing"

func TestATSocialInsurance(t *testing.T) {
	d, ok := Get("at-social-insurance")
	if !ok {
		t.Fatal("not registered")
	}
	cases := []struct {
		why   string
		value string
		want  bool
	}{
		{"valid", "2456150372", true},
		{"valid with the usual space", "2456 150372", true},
		{"leap day", "3818290204", true},
		{"fictitious month 14", "6627071475", true},
		{"wrong check digit", "2457150372", false},
		{"remainder 10 is never issued", "3810290203", false},
		{"29 February in a year that is not a leap year", "3812290203", false},
		{"month 16 is not used", "3813291603", false},
		{"running number below 100", "0450150372", false},
		{"too short", "245615037", false},
		{"too long", "24561503720", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.value)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.why, got, c.want)
		}
	}
	if got := d.Normalize("2456 150372"); got != "2456150372" {
		t.Errorf("normalize kept the separator")
	}
	if m := d.Mask("2456150372"); m != "••••••••••" {
		t.Errorf("mask does not hide the value")
	}
	if !d.Formatted.MatchString("2456 150372") || d.Formatted.MatchString("2456150372") {
		t.Errorf("the usual format is a group of four and a group of six")
	}
}
