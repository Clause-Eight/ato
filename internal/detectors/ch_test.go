package detectors

import "testing"

// The values below are made up so that they pass (or fail) the published rules; they belong to nobody.

func TestSwissAHVRules(t *testing.T) {
	checkCases(t, "ch-ahv", map[string]bool{
		"7560000000019":    true,
		"756.0000.0000.19": true,
		"7560000000018":    false, // wrong check digit
		"7570000000019":    false, // not the Swiss prefix
		"756000000001":     false,
	})
	d, _ := Get("ch-ahv")
	if !d.Formatted.MatchString("756.0000.0000.19") || d.Formatted.MatchString("7560000000019") {
		t.Errorf("the dotted format must raise confidence, the bare digits must not")
	}
	if !d.Samples["7561234567897"] || !d.Validate("7561234567897") {
		t.Errorf("the published example must be valid and marked as a sample")
	}
}
