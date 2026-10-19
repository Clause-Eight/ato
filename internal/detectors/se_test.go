package detectors

import "testing"

func TestSwedishPersonnummer(t *testing.T) {
	nordicBalticCases(t, "se-personnummer", map[string]string{
		"ten digits with dash":         "850712-1237",
		"ten digits":                   "8507121237",
		"over 100, plus sign":          "850712+1237",
		"twelve digits":                "198507121237",
		"twelve digits with dash":      "19850712-1237",
		"coordination number":          "850772-1234",
		"29 February 2000, short form": "000229-4569",
	}, map[string]string{
		"wrong check digit":           "850712-1238",
		"month 13":                    "851312-1237",
		"day 32":                      "850732-1237",
		"coordination day 92":         "850792-1237",
		"century 17":                  "178507121237",
		"29 February 1900, long form": "190002294569",
		"birth number 000":            "850712-0007",
		"eleven digits":               "08507121237",
	})
	d, _ := Get("se-personnummer")
	if got := d.Normalize("850712+1237"); got != "8507121237" {
		t.Errorf("normalised to %q", got)
	}
	if !d.Formatted.MatchString("850712-1237") || d.Formatted.MatchString("8507121237") {
		t.Error("the dash is the usual format")
	}
}
