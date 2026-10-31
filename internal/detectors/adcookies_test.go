package detectors

import "testing"

func TestGAClientID(t *testing.T) {
	d := registered(t, "ga-client-id")
	checkCases(t, "ga-client-id", map[string]bool{
		"GA1.2.1234567890.1700000000": true,
		"GA1.1.98765.1500644855":      true,
		"GA1.2.2147483647.1600000000": true,
		"GA1.2.2147483648.1600000000": false, // above the 31-bit range
		"GA1.2.0.1600000000":          false,
		"GA1.0.1234567890.1700000000": false, // no domain depth
		"GA1.2.1234567890.1100000000": false, // 2004, before the cookie existed
		"GA1.2.1234567890.9900000000": false, // far in the future
	})
	if d.Pattern.MatchString("version GA1.2.3.4") || d.Pattern.MatchString("GA1.2.1234567890.170000000") {
		t.Errorf("a version or a time in fewer than ten digits is not a client ID")
	}
	if m := d.Mask("GA1.2.1234567890.1700000000"); m[:4] != "GA1." || len([]rune(m)) != 27 {
		t.Errorf("the mask must keep the prefix and the length")
	}
}

func TestMetaPixelCookie(t *testing.T) {
	checkCases(t, "meta-pixel-cookie", map[string]bool{
		"fb.1.1596403881668.1116446470":                 true, // _fbp
		"fb.0.1700000000123.42424242":                   true,
		"fb.1.1600000000000.ZxQwErTy-UiOp_AsDfGhJkLmNb": true,  // _fbc with a click ID
		"fb.2.1400000000000.1116446470":                 false, // 2014, before the first-party cookie
		"fb.1.9900000000000.1116446470":                 false, // in the future
		"fb.1.1596403881668.000000":                     false,
		"fb.1.1596403881668.123456789012345678901":      false, // longer than any random number
	})
	d := registered(t, "meta-pixel-cookie")
	if d.Pattern.MatchString("fb.1.159640388166.1116446470") {
		t.Errorf("a creation time in seconds is not the cookie's format")
	}
}

// registered returns a detector whether or not it is on by default.
func registered(t *testing.T, name string) Detector {
	t.Helper()
	d, ok := Get(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	return d
}
