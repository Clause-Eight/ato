package detectors

import "testing"

func TestIPAddressKeepsOnlyPublicAddresses(t *testing.T) {
	d, _ := Get("ip-address")
	cases := map[string]bool{
		"85.120.33.17":                true,
		"212.58.4.200":                true,
		"2a00:1450:4001:81c::1":       true,
		"2a02:8070:a1b2:c3d4:1:2:3:4": true,
		"10.1.2.3":                    false, // private
		"172.20.0.5":                  false,
		"192.168.1.10":                false,
		"127.0.0.1":                   false, // loopback
		"169.254.10.1":                false, // link-local
		"100.64.1.1":                  false, // shared address space (CGNAT)
		"198.18.0.9":                  false, // benchmarking
		"192.0.2.10":                  false, // documentation
		"198.51.100.7":                false,
		"203.0.113.99":                false,
		"224.0.0.251":                 false, // multicast
		"240.1.2.3":                   false, // reserved
		"255.255.255.255":             false,
		"0.0.0.0":                     false,
		"85.120.33.0":                 false, // a network, not a host
		"8.8.8.8":                     false, // public resolver
		"1.2.3.4":                     false, // placeholder
		"2001:db8::1":                 false, // documentation
		"2001:0:4136:e378::1":         false, // Teredo, in 2001::/23
		"2002:c000:204::1":            false, // 6to4
		"2024:01:02":                  false, // a timestamp, not an address
		"fe80::1":                     false,
		"version 85.120.33.17":        false,
		"v85.120.33.17":               false,
		"256.1.2.3":                   false,
	}
	for v, want := range cases {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("ip case %q: got %v, want %v", v, got, want)
		}
	}
	for _, line := range []string{`"version": "85.120.33.17"`, "release 4.2.1.3 shipped"} {
		for _, m := range d.Pattern.FindAllString(line, -1) {
			if d.Validate(d.Normalize(m)) {
				t.Errorf("a version or zero-padded number is accepted as an address")
			}
		}
	}
	if !d.Needs.Check("host 85.120.33.17") || !d.Needs.Check("[2a00::1]") || d.Needs.Check("timeout: 30s") {
		t.Errorf("prefilter")
	}
}

func TestMACAddress(t *testing.T) {
	d, _ := Get("mac-address")
	cases := map[string]bool{
		"02:1a:2b:3c:4d:5e": true,
		"02-1A-2B-3C-4D-5E": true,
		"021a.2b3c.4d5e":    true,
		"02:1a-2b:3c:4d:5e": false, // mixed separators
		"00:00:00:00:00:00": false,
		"ff:ff:ff:ff:ff:ff": false,
		"01:00:5e:00:00:fb": false, // multicast
		"33:33:00:00:00:01": false,
		"02:1a:2b:3c:4d":    false,
	}
	for v, want := range cases {
		if got := d.Validate(d.Normalize(v)); got != want {
			t.Errorf("mac case %q: got %v, want %v", v, got, want)
		}
	}
	if !d.NeedsKeyword || d.OnByDefault {
		t.Errorf("mac-address must need a keyword and be off by default")
	}
	if !d.Needs.Check("hw 02:1a:2b") || !d.Needs.Check("021a.2b3c.4d5e") || d.Needs.Check("timeout: 30s") {
		t.Errorf("prefilter")
	}
}
