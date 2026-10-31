package detectors

import "testing"

func TestDanishCPR(t *testing.T) {
	nordicBalticCases(t, "dk-cpr", map[string]string{
		"1900s with dash":                 "120785-1234",
		"1900s without dash":              "1207851234",
		"29 February 2000 (seventh 4)":    "290200-4123",
		"29 February 2008 (seventh 5)":    "290208-5123",
		"1937 or later with seventh 9":    "010160-9123",
		"no modulus-11 needed since 2007": "010107-4000",
	}, map[string]string{
		"29 February 1900 (seventh 1)": "290200-1123",
		"29 February 1858 (seventh 5)": "290258-5123",
		"day 32":                       "320185-1234",
		"month 13":                     "121385-1234",
		"nine digits":                  "120785-123",
	})
	d, _ := Get("dk-cpr")
	if d.OnByDefault || !d.NeedsKeyword {
		t.Error("the CPR number has no reliable check digit: off by default and keyword-gated")
	}
	if got := d.Normalize("120785-1234"); got != "1207851234" {
		t.Errorf("normalised to %q", got)
	}
	if !d.Formatted.MatchString("120785-1234") || d.Formatted.MatchString("1207851234") {
		t.Error("the dash is the usual format")
	}
}
