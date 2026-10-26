package detectors

import "testing"

// Every value in this file is synthetic.

func TestCyprusIDCard(t *testing.T) {
	d, _ := Get("cy-id-card")
	if !d.NeedsKeyword {
		t.Fatal("a format-only number must need a keyword")
	}
	for _, c := range []struct {
		name string
		in   string
		want int
	}{
		{"six digits", "id 482913", 6},
		{"eight digits", "id 48291375", 8},
		{"five digits", "id 48291", 0},
	} {
		if got := len(d.Pattern.FindString(c.in)); got != c.want {
			t.Errorf("%s: found %d characters, want %d", c.name, got, c.want)
		}
	}
	for _, c := range []struct {
		name string
		line string
		want bool
	}{
		{"Greek name of the card", "Αριθμός Δελτίου Ταυτότητας: 482913", true},
		{"Greek abbreviation", "ΑΔΤ 482913", true},
		{"English", "cyprus_id = 482913", true},
		{"Turkish", "kimlik no 482913", true},
		{"no card name", "order 482913", false},
	} {
		if got := d.Needs.Admits(c.line, 6); got != c.want {
			t.Errorf("%s: prefilter got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCYTIC(t *testing.T) {
	d, _ := Get("cy-tic")
	cases := map[string]bool{
		"05780357D": true,
		"00704143O": true,
		"66456382W": true, // the range issued since 2023
		"95434894C": true,
		"05780357F": false, // wrong check letter
		"12321542B": false, // a company's code
		"34062429P": false,
		"05780357d": false, // lower case
		"0578035D":  false, // too short
	}
	for v, want := range cases {
		if got := d.Validate(v); got != want {
			t.Errorf("case %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
	if !d.Needs.Admits("tic: 05780357D", 8) || d.Needs.Admits("tic: 05780357", 8) || d.Needs.Admits("05780357 d", 8) {
		t.Error("the prefilter must look for a digit followed by a capital")
	}
}
