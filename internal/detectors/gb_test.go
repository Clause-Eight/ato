package detectors

import "testing"

// The values below are made up so that they pass (or fail) the published rules; they belong to nobody.

func TestUKNINORules(t *testing.T) {
	checkCases(t, "gb-nino", map[string]bool{
		"AB123456C":     true,
		"AB 12 34 56 C": true,
		"AB123456":      false, // suffix left out: too much like other codes
		"AB123456E":     false,
		"DA123456A":     false, // D is never a prefix letter
		"AO123456A":     false, // O is never the second letter
		"GB123456A":     false, // not allocated
		"TN123456A":     false,
		"ZZ123456A":     false,
		"AB12345C":      false,
		"ab123456c":     false,
	})
	d, _ := Get("gb-nino")
	if d.Pattern.FindString("DA123456A") != "" || d.Pattern.FindString("AO123456A") != "" {
		t.Errorf("the pattern must leave out letters that are never used")
	}
	if !d.Needs.Admits("nino: AB 123456 C", 6) || d.Needs.Admits("const ab = 123456", 6) {
		t.Errorf("prefilter does not look for two capitals before a digit")
	}
}

func TestUKNHSRules(t *testing.T) {
	checkCases(t, "gb-nhs", map[string]bool{
		"4000000012":   true,
		"400 000 0012": true,
		"650-000-1249": true,
		"3500000010":   true,  // Northern Ireland
		"0101901232":   true,  // Scottish CHI number with a date of birth
		"0013901230":   false, // CHI range without a valid date
		"4000000013":   false, // wrong check digit
		"9990000018":   false, // reserved for tests
		"8500001232":   false, // not an issued range
		"400000001":    false,
	})
}
