package detectors

import "testing"

func TestEircode(t *testing.T) {
	d := registered(t, "ie-eircode")
	checkCases(t, "ie-eircode", map[string]bool{
		"D02 X285": true,
		"D02X285":  true,
		"D6W 4KT7": true, // the one key with a letter in third place
		"T12 AC4E": true,
		"Y35 0F9W": true,
		"A65 F4E2": false, // Eircode's own example: A65 is not a routing key
		"D19 X285": false, // Dublin has no D19
		"B12 X285": false, // B is not used
		"D02 X2B5": false, // B is not in the unique identifier's alphabet
		"D02 0000": false, // a placeholder
		"D02 X28":  false,
	})
	if len(eircodeRoutingKeys) != 139 {
		t.Errorf("Eircode has 139 routing keys, the list has %d", len(eircodeRoutingKeys))
	}
	if !d.Formatted.MatchString("D02 X285") || d.Formatted.MatchString("D02X285") {
		t.Errorf("the spaced form is the usual one")
	}
	if !d.Needs.Check("eircode: D02 X285") || d.Needs.Check("press F11 to open") || d.Needs.Check("timeout: 30s") {
		t.Errorf("prefilter")
	}
}
