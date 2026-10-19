package detectors

import (
	"testing"
)

func TestGPSCoordinates(t *testing.T) {
	d := detector(t, "gps-coordinates")
	if !d.NeedsKeyword {
		t.Error("gps-coordinates must need a keyword")
	}
	checkCases(t, "gps-coordinates", map[string]bool{
		"44.4268, 26.1025":              true,
		"44.4268,26.1025":               true,
		"-33.8688 151.2093":             true,
		"lat=44.42680 lng=26.10250":     true,
		`lat": 44.4268, "lng": 26.1025`: true,
		"44.426, 26.1025":               false, // three decimals
		"91.0000, 26.1025":              false,
		"44.4268, 181.1025":             false,
		"0.0000, 0.0000":                false,
		"45.0000, 26.0000":              false,
	})
	for i, s := range []string{`{"lat": 44.4268, "lng": 26.1025}`, "gps lat=-12.34567&lon=-76.54321", "at 48.85837, 2.29448 today"} {
		got := d.Pattern.FindString(s)
		if got == "" || !d.Validate(got) {
			t.Errorf("gps case %d is not found", i)
		}
	}
}
