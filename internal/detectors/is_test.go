package detectors

import "testing"

// The values below are made up so that they pass (or fail) the published rules; they belong to nobody.

func TestIcelandicKennitalaRules(t *testing.T) {
	checkCases(t, "is-kennitala", map[string]bool{
		"0101902079":  true,
		"010190-2079": true,
		"1502853489":  true,
		"0101902089":  false, // wrong check digit
		"0101902077":  false, // century digit 7 is not used
		"0101902070":  false, // born in 2090
		"2902002020":  true,  // 29 February 2000
		"2902002029":  false, // 29 February 1900 is not a date
		"4101902069":  false, // a company: day plus 40
		"010190207":   false,
	})
}
