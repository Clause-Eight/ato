package detectors

import (
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// The values below are made up so that they pass (or fail) the published rules; they belong to nobody.

func TestEEAPackIsOptIn(t *testing.T) {
	other := map[string]bool{}
	for _, cc := range domain.OtherCountries {
		other[cc] = true
	}
	for _, d := range All() {
		if other[d.Country] && d.OnByDefault {
			t.Errorf("%s is for a country outside the EU and must be off by default", d.Name)
		}
	}
	if d, _ := Get("no-fnr"); !other[d.Country] {
		t.Errorf("no-fnr must use a country code from domain.OtherCountries")
	}
}

func TestNorwegianFnrRules(t *testing.T) {
	checkCases(t, "no-fnr", map[string]bool{
		"01019012480":  true,
		"010190 12480": true,
		"15028545670":  true,
		"01019012481":  false, // second check digit wrong
		"01019012490":  false, // first check digit wrong
		"41019012474":  true,  // D-number: day plus 40
		"01419012382":  true,  // H-number: month plus 40
		"01159012499":  false, // month 15
		"01859012406":  false, // synthetic test number: month plus 80
		"01010551346":  true,  // individual number 500-999 with YY < 40: born in 2005
		"01014560013":  false, // individual number 600 with YY 45 has no century
		"29020051386":  true,  // 29 February 2000
		"29020012461":  false, // 29 February 1900 is not a date
		"0101901248":   false,
	})
}
