package detectors

import "testing"

// luhnComplete appends the Luhn check digit to body, computed here for the test.
func luhnComplete(body string) string {
	for c := byte('0'); c <= '9'; c++ {
		if Luhn(body + string(c)) {
			return body + string(c)
		}
	}
	return ""
}

func TestIMEI(t *testing.T) {
	d, _ := Get("imei")
	good := luhnComplete("35123456789012")
	if !d.Validate(d.Normalize(good)) {
		t.Fatalf("a valid IMEI is rejected")
	}
	formatted := good[:2] + "-" + good[2:8] + "-" + good[8:14] + "-" + good[14:]
	if d.Normalize(formatted) != good || !d.Formatted.MatchString(formatted) || d.Pattern.FindString("imei "+formatted) != formatted {
		t.Errorf("the usual format is not handled")
	}
	bad := map[string]string{
		"wrong check digit":      good[:14] + string('0'+(good[14]-'0'+1)%10),
		"test reporting body":    luhnComplete("00123456789012"),
		"unknown body":           luhnComplete("77123456789012"),
		"Amex card number":       luhnComplete("37123456789012"),
		"fourteen digits":        good[:14],
		"sixteen digits":         luhnComplete("351234567890123"),
		"Maestro card number 50": luhnComplete("50123456789012"),
	}
	for name, v := range bad {
		if d.Validate(d.Normalize(v)) {
			t.Errorf("accepted: %s", name)
		}
	}
	if d.Mask(good) == good || len([]rune(d.Mask(good))) != len(good) {
		t.Errorf("mask")
	}
}

func TestAdvertisingID(t *testing.T) {
	d, _ := Get("advertising-id")
	for v, want := range map[string]bool{
		"6D92078A-8246-4BA4-AE5B-76104861E7DC": true,
		"38400000-8cf0-11bd-b23e-10b96e40000d": true,
		"00000000-0000-0000-0000-000000000000": false, // tracking not allowed
		"ffffffff-ffff-ffff-ffff-ffffffffffff": false,
		"6d92078a-8246-4ba4-ae5b-76104861e7d":  false,
		"6d92078a82464ba4ae5b76104861e7dc":     false,
	} {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("advertising id case %d characters: got %v, want %v", len(v), got, want)
		}
	}
	if !d.NeedsKeyword || d.OnByDefault {
		t.Errorf("advertising-id must need a keyword and be off by default")
	}
	if !d.Needs.Check("idfa=6D92078A-8246-4BA4") || d.Needs.Check("a-b-c-d") {
		t.Errorf("prefilter")
	}
}
