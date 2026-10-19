package detectors

import "testing"

func TestFinnishHETU(t *testing.T) {
	nordicBalticCases(t, "fi-hetu", map[string]string{
		"1900s with dash":        "120785-1239",
		"1900s with sign Y":      "120785Y1239",
		"1900s with sign U":      "120785U1239",
		"2000s with sign A":      "010105A2349",
		"2000s with sign F":      "010105F2349",
		"29 February 2000":       "290200A345E",
		"1800s with plus":        "120785+1239",
		"temporary number range": "120785-900B",
	}, map[string]string{
		"wrong check character":       "120785-1238",
		"29 February 1900":            "290200-345E",
		"individual number 001":       "010180-0014",
		"unknown sign G":              "120785G1239",
		"lower-case sign":             "010105a2349",
		"check character G":           "120785-123G",
		"ten characters":              "120785-123",
		"day 32":                      "320785-1239",
		"letter in individual number": "120785-1A39",
	})
	d, _ := Get("fi-hetu")
	if d.Pattern.MatchString("120785-123g") {
		t.Error("lower-case check characters are not matched")
	}
}
