package detectors

import "testing"

func checkCases(t *testing.T, name string, cases map[string]bool) {
	t.Helper()
	d, ok := Get(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	for v, want := range cases {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("%s: case of %d characters, valid=%v: got %v", name, len(v), want, got)
		}
	}
}
