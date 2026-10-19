package detectors

import "testing"

// nordicBalticCases runs a detector's pattern, normalisation and validator over valid and invalid inputs.
func nordicBalticCases(t *testing.T, name string, valid, invalid map[string]string) {
	t.Helper()
	d, ok := Get(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	for why, v := range valid {
		if got := d.Pattern.FindString(v); got != v {
			t.Errorf("%s: %s is not found whole", name, why)
		}
		n := d.Normalize(v)
		if !d.Validate(n) {
			t.Errorf("%s: %s is rejected", name, why)
		}
		if m := d.Mask(n); len([]rune(m)) != len([]rune(n)) || m == n {
			t.Errorf("%s: %s is not masked", name, why)
		}
	}
	for why, v := range invalid {
		if d.Validate(d.Normalize(v)) {
			t.Errorf("%s: %s is accepted", name, why)
		}
	}
}

func TestEstonianPersonalCode(t *testing.T) {
	nordicBalticCases(t, "ee-isikukood", map[string]string{
		"1900s man":                   "38507121238",
		"2000s woman, 29 February":    "60002294569",
		"second-pass check digit":     "38507120093",
		"both passes 10, check digit": "51003151050",
	}, map[string]string{
		"wrong check digit":      "38507121239",
		"century digit 7":        "78507121238",
		"century digit 0":        "08507121238",
		"29 February 1900":       "40002294567",
		"month 13":               "38513121238",
		"ten digits":             "3850712123",
		"letters":                "3850712123A",
		"second pass is skipped": "38507120090",
	})
}

func TestLithuanianPersonalCode(t *testing.T) {
	nordicBalticCases(t, "lt-asmens-kodas", map[string]string{
		"1900s man":                "38507121238",
		"2000s woman, 29 February": "60002294569",
	}, map[string]string{
		"wrong check digit": "38507121239",
		"century digit 9":   "98507121238",
		"day 00":            "38507001238",
	})
	ee, _ := Get("ee-isikukood")
	lt, _ := Get("lt-asmens-kodas")
	if contains(lt.MatchWords, "isikukood") || contains(ee.MatchWords, "asmens kodas") {
		t.Error("one detector's own name must not raise the other")
	}
}
