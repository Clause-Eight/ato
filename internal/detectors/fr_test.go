package detectors

import "testing"

// wantValidity runs each case through the detector's Normalize and Validate. Failures print the masked value.
func wantValidity(t *testing.T, name string, cases map[string]bool) {
	t.Helper()
	d, ok := Get(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	for in, want := range cases {
		if got := d.Validate(d.Normalize(in)); got != want {
			t.Errorf("%s: case %q: got %v, want %v", name, d.Mask(d.Normalize(in)), got, want)
		}
	}
}

// wantPatternFinds checks that the detector's pattern finds want in text, whole.
func wantPatternFinds(t *testing.T, name string, text, want string) {
	t.Helper()
	d, _ := Get(name)
	if got := d.Pattern.FindString(text); got != want {
		t.Errorf("%s: pattern found %d characters, want %d", name, len(got), len(want))
	}
}

func TestFrenchNIR(t *testing.T) {
	wantValidity(t, "fr-nir", map[string]bool{
		"184037512304516":       true,
		"1 84 03 75 123 045 16": true,
		"291129920401715":       true,  // born abroad (99), continent 2
		"173029710501233":       true,  // overseas (97), collectivity 1
		"169012A21200190":       true,  // Corsica 2A
		"169012B21200120":       true,  // Corsica 2B
		"2 77 06 2B 033 120 59": true,  // Corsica 2B, spaced
		"145217505600101":       true,  // month 21, incomplete civil status
		"184037512304517":       false, // wrong key
		"169012B21200190":       false, // 2B instead of 2A changes the key
		"384037512304513":       false, // sex 3
		"184137512304579":       false, // month 13
		"184030012304504":       false, // department 00
		"184037500004520":       false, // commune 000
		"184037512300061":       false, // order 000
		"173029790501289":       false, // overseas collectivity 9
		"291129960401743":       false, // continent 6
		"18403751230451":        false, // 14 characters
	})
	wantPatternFinds(t, "fr-nir", "nir: 1 84 03 75 123 045 16;", "1 84 03 75 123 045 16")
	d, _ := Get("fr-nir")
	if d.Formatted.MatchString("184037512304516") || !d.Formatted.MatchString("1 84 03 75 123 045 16") {
		t.Error("only the spaced form is the usual format")
	}
	if m := d.Mask("184037512304516"); m != "•••••••••••••••" {
		t.Errorf("mask: %q", m)
	}
}

func TestFrenchSPI(t *testing.T) {
	wantValidity(t, "fr-spi", map[string]bool{
		"1234567890066":     true,
		"12 34 567 890 066": true,
		"0012345678429":     true, // key below 100 is padded
		"2999999999248":     true,
		"3000000001250":     true,
		"1234567890067":     false, // wrong key
		"4234567890315":     false, // first digit 4
		"123456789066":      false, // 12 digits
		"12345678900660":    false, // 14 digits
	})
	wantPatternFinds(t, "fr-spi", "numéro fiscal: 12 34 567 890 066", "12 34 567 890 066")
	d, _ := Get("fr-spi")
	if !d.Samples["3023217600053"] || !d.Validate("3023217600053") {
		t.Error("the DGFiP specimen must validate and be marked as a sample")
	}
	if !d.Formatted.MatchString("12 34 567 890 066") || d.Formatted.MatchString("1234567890066") {
		t.Error("only the grouped form is the usual format")
	}
}
