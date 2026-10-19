package detectors

import (
	"net/netip"
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// IP addresses are personal data when the holder can be identified with the help of the access provider:
// CJEU C-582/14 Breyer, 19 October 2016 (https://curia.europa.eu/juris/liste.jsf?num=C-582/14). Only public
// addresses are reported: IPv4 octets 0-255 without leading zeros, and IPv6 in the global unicast space
// 2000::/3 (https://www.iana.org/assignments/ipv6-address-space). Every block in the IANA IPv4 and IPv6
// special-purpose registries is left out (private, loopback, link-local, shared CGNAT 100.64/10, benchmarking,
// documentation, multicast, reserved; https://www.iana.org/assignments/iana-ipv4-special-registry and
// https://www.iana.org/assignments/iana-ipv6-special-registry), as are network addresses ending in .0 and the
// well-known public resolvers, which are configuration rather than a person. A version string written
// after "version", "v", "build" or "release" is not an address. Addresses fill logs and configuration, so the
// detector is off by default.
//
// MAC addresses (IEEE 802 EUI-48): six hex pairs separated by colons or dashes, or three dotted groups of four.
// The all-zero, broadcast and group (multicast) addresses name no device. There is no check digit, so a keyword
// is required (https://standards.ieee.org/products-programs/regauth/).
func init() {
	Register(Detector{
		Name:        "ip-address",
		Label:       "IP address",
		Country:     "EU",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`(?i:\b(?:v|ver|version|build|release|rev)["':= ]{0,4})?` + ipv4Re + `|2[0-9A-Fa-f]{3}(?::[0-9A-Fa-f]{0,4}){2,7}`),
		Normalize:   strings.ToLower,
		Validate:    validIP,
		Keywords:    []string{"ip", "ipv4", "ipv6", "ip address", "ip addr", "ipaddr", "ipaddress", "remote addr", "client ip", "x forwarded for", "x real ip", "src ip", "source ip"},
		Base:        domain.Low,
		Validation:  "a public address outside the IANA special-purpose blocks",
		Needs:       Needs{Check: maybeIP},
	})
	Register(Detector{
		Name:         "mac-address",
		Label:        "MAC address",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`[0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}|[0-9A-Fa-f]{2}(?:-[0-9A-Fa-f]{2}){5}|[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}`),
		Validate:     validMAC,
		Keywords:     []string{"mac", "mac address", "macaddr", "macaddress", "hwaddr", "hw addr", "hardware address", "bssid", "ether"},
		Base:         domain.Low,
		Validation:   "a device (unicast) address, not all-zero or broadcast",
		NeedsKeyword: true,
		Needs:        Needs{Check: maybeMAC},
	})
}

const ipv4Octet = `(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)`

const ipv4Re = ipv4Octet + `(?:\.` + ipv4Octet + `){3}`

// ipv4Special are the blocks of the IANA IPv4 special-purpose registry, plus multicast 224/4.
var ipv4Special = prefixes("0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
	"192.0.0.0/24", "192.0.2.0/24", "192.31.196.0/24", "192.52.193.0/24", "192.88.99.0/24", "192.168.0.0/16",
	"192.175.48.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4")

// ipv6Special are the IANA IPv6 special-purpose blocks inside the global unicast space 2000::/3.
var ipv6Special = prefixes("2001::/23", "2001:db8::/32", "2002::/16", "2620:4f:8000::/48", "3fff::/20")

// wellKnownIPs are public resolvers and the usual placeholder, found in configuration and examples.
var wellKnownIPs = map[string]bool{
	"1.1.1.1": true, "1.0.0.1": true, "8.8.8.8": true, "8.8.4.4": true, "9.9.9.9": true, "149.112.112.112": true,
	"208.67.222.222": true, "208.67.220.220": true, "1.2.3.4": true,
}

func prefixes(ps ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ps))
	for i, p := range ps {
		out[i] = netip.MustParsePrefix(p)
	}
	return out
}

func validIP(s string) bool {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}
	a, err := netip.ParseAddr(s)
	if err != nil || a.Zone() != "" {
		return false
	}
	special := ipv6Special
	if a.Is4() {
		if wellKnownIPs[s] || a.As4()[3] == 0 || a == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
			return false
		}
		special = ipv4Special
	} else if a.Is4In6() || !netip.MustParsePrefix("2000::/3").Contains(a) {
		return false
	}
	for _, p := range special {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// maybeIP looks for a digit, a dot and a digit (IPv4) or four hex characters starting with 2 and a colon (IPv6).
func maybeIP(line string) bool {
	for i := 1; i+1 < len(line); i++ {
		switch line[i] {
		case '.':
			if isASCIIDigit(line[i-1]) && isASCIIDigit(line[i+1]) {
				return true
			}
		case ':':
			if i >= 4 && line[i-4] == '2' && isHex(line[i-3]) && isHex(line[i-2]) && isHex(line[i-1]) {
				return true
			}
		}
	}
	return false
}

func validMAC(s string) bool {
	var hex string
	switch {
	case len(s) == 17 && (strings.Count(s, ":") == 5 || strings.Count(s, "-") == 5):
		hex = strings.NewReplacer(":", "", "-", "").Replace(s)
	case len(s) == 14 && strings.Count(s, ".") == 2:
		hex = strings.ReplaceAll(s, ".", "")
	default:
		return false
	}
	if len(hex) != 12 {
		return false
	}
	hex = strings.ToLower(hex)
	for i := 0; i < len(hex); i++ {
		if !isHex(hex[i]) {
			return false
		}
	}
	if hex == "000000000000" || hex == "ffffffffffff" {
		return false
	}
	return hexValue(hex[1])&1 == 0
}

// maybeMAC looks for two hex pairs joined by a colon or dash, or a dotted group of four.
func maybeMAC(line string) bool {
	for i := 2; i+2 < len(line); i++ {
		c := line[i]
		if (c == ':' || c == '-') && isHex(line[i-2]) && isHex(line[i-1]) && isHex(line[i+1]) && isHex(line[i+2]) {
			return true
		}
		if c == '.' && i >= 4 && i+4 < len(line) && isHex(line[i-4]) && isHex(line[i-1]) && isHex(line[i+1]) && isHex(line[i+4]) {
			return true
		}
	}
	return false
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool { return isASCIIDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func hexValue(c byte) byte {
	if isASCIIDigit(c) {
		return c - '0'
	}
	return c - 'a' + 10
}
