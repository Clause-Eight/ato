package detectors

import "testing"

func TestLatvianPersonalCode(t *testing.T) {
	nordicBalticCases(t, "lv-personal-code", map[string]string{
		"1900s with dash":       "120785-12347",
		"1900s without dash":    "12078512347",
		"2000s":                 "010101-20001",
		"29 February 2000":      "290200-22346",
		"check digit from 1101": "120785-12363",
		"another serial, 1900s": "120785-12355",
	}, map[string]string{
		"wrong check digit":          "120785-12348",
		"remainder 10 is not issued": "010101-20010",
		"century digit 3":            "120785-32347",
		"29 February 1900":           "290200-12340",
		"day 32":                     "320785-12347",
		"ten digits":                 "120785-1234",
	})
	nordicBalticCases(t, "lv-personal-code-new", map[string]string{
		"with dash":    "320000-12345",
		"without dash": "32000012345",
	}, map[string]string{
		"prefix 33":     "330000-12345",
		"twelve digits": "320000-123456",
	})
	d, _ := Get("lv-personal-code-new")
	if !d.NeedsKeyword {
		t.Error("codes without a check digit need a keyword")
	}
	old, _ := Get("lv-personal-code")
	if old.Validate(old.Normalize("320000-12345")) {
		t.Error("a new code must not validate as an old one")
	}
}
