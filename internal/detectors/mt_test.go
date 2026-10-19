package detectors

import "testing"

// Every value in this file is synthetic.

func TestMaltaIDCard(t *testing.T) {
	d, _ := Get("mt-id-card")
	if !d.NeedsKeyword {
		t.Fatal("a format-only number must need a keyword")
	}
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"Maltese birth registered in the 1900s", "0471985M", true},
		{"Gozitan birth registered in the 1900s", "1290377G", true},
		{"Maltese birth registered after 2000", "0042306L", true},
		{"leading zeros left out", "42306L", true},
		{"residence permit holder", "0123456A", true},
		{"space before the letter", "0471985 M", true},
		{"after-2000 letter with a year still to come", "0042399L", false},
		{"unknown letter", "0471985X", false},
		{"too long", "12345678M", false},
		{"no letter", "0471985", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if n := d.Normalize("0471985 M"); n != "0471985M" {
		t.Errorf("normalize: got %d characters", len(n))
	}
	if !d.Formatted.MatchString("0471985M") || d.Formatted.MatchString("42306L") {
		t.Error("only the full seven-digit form is the usual format")
	}
	if d.Needs.Admits("memory: 512 MB", 3) != true || d.Needs.Admits("no letter after 123 here", 3) {
		t.Error("the prefilter needs a digit followed by a card letter")
	}
}
