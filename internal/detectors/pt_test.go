package detectors

import "testing"

// Every value in this file is synthetic, with check digits computed by hand outside this package.

func TestPortugueseNIF(t *testing.T) {
	d, _ := Get("pt-nif")
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"individual starting with 2", "217539041", true},
		{"individual starting with 3", "304188263", true},
		{"non-resident starting with 45", "451200675", true},
		{"written in groups of three", "217 539 041", true},
		{"wrong check digit", "217539042", false},
		{"company prefix 5 is not personal data", "543210987", false},
		{"prefix 4 without 5", "417539041", false},
		{"too short", "21753904", false},
		{"too long", "2175390410", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := d.Pattern.FindString("nif: 543210987"); got != "" {
		t.Errorf("the pattern should not start on a company prefix, found %d characters", len(got))
	}
	if m := d.Mask(d.Normalize("217 539 041")); m != "•••••••••" {
		t.Errorf("mask: got %q", m)
	}
}

func TestPortugueseCitizenCard(t *testing.T) {
	d, _ := Get("pt-citizen-card")
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"compact", "174930268ZZ5", true},
		{"printed with spaces", "06351842 7 ZY4", true},
		{"wrong check digit", "174930268ZZ6", false},
		{"changed version letter", "174930268ZY5", false},
		{"changed civil number", "174930278ZZ5", false},
		{"letter outside the version", "17493026AZZ5", false},
		{"too short", "17493026ZZ5", false},
		{"lower case", "174930268zz5", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.in)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := d.Pattern.FindString("x 123456789012 y"); got != "" {
		t.Errorf("twelve plain digits should not be a candidate, found %d characters", len(got))
	}
	if !d.Formatted.MatchString("06351842 7 ZY4") || d.Formatted.MatchString("063518427ZY4") {
		t.Error("the printed format with spaces should raise confidence, the compact one should not")
	}
	if n := d.Normalize("06351842 7 ZY4"); n != "063518427ZY4" {
		t.Errorf("normalize: got %d characters", len(n))
	}
}
