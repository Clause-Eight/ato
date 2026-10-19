package detectors

import "testing"

func TestBSN(t *testing.T) {
	d, _ := Get("nl-bsn")
	cases := []struct {
		name, in string
		want     bool
	}{
		{"plain", "135239576", true},
		{"written with dots", "8316.53.942", true},
		{"leading zero", "012345672", true},
		{"wrong check digit", "135239577", false},
		{"another last digit", "135239570", false},
		{"all zeros", "000000000", false},
		{"eight digits", "13523957", false},
		{"ten digits", "1352395760", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := d.Normalize("8316.53.942"); got != "831653942" {
		t.Errorf("normalised to %d characters", len(got))
	}
	if m := d.Mask("135239576"); m != "•••••••••" {
		t.Errorf("mask %q", m)
	}
}
