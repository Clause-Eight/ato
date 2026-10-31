package detectors

import "testing"

func TestWhat3words(t *testing.T) {
	d := registered(t, "what3words")
	checkCases(t, "what3words", map[string]bool{
		"///kalo.mitas.renvo":                     true,
		"https://w3w.co/kalo.mitas.renvo":         true,
		"https://what3words.com/kalo.mitas.renvo": true,
		"w3w.co/kalo.mitas.renvo":                 true,
		"///ápfel.straße.öl":                      true,  // any script
		"///かろ・みた・れんぼ":                            true,  // with the ideographic stop
		"kalo.mitas.renvo":                        false, // three dotted words alone are any name in code
		"////kalo.mitas.renvo":                    false,
		"dns:///storage.example.com":              false, // a URL with an empty host
		"file:///kalo.mitas.renvo":                false,
		"///envoy.config.route.v3":                false, // more than three parts
		"///metadata.google.internal":             false,
		"///kalo.mitas":                           false,
	})
	for _, line := range []string{
		`target: "dns:///storage.example.com:443"`,
		`name = "xdstp:///envoy.config.listener.v3.Listener/x"`,
		`url: "file:///kalo.mitas.renvo"`,
		`/// kalo.mitas.renvo is a doc comment`,
	} {
		if m := d.Pattern.FindString(line); m != "" && d.Validate(m) {
			t.Errorf("a URL scheme or a doc comment is not an address")
		}
	}
	if m := d.Mask("///kalo.mitas.renvo"); m[:3] != "///" || len([]rune(m)) != 19 {
		t.Errorf("the mask must keep the prefix and the length")
	}
}
