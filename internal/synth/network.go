package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("ip-address", func(r *rand.Rand) string {
		if r.Intn(3) == 0 {
			// Inside the regional registries' global unicast blocks, away from 2001::/23, 2001:db8::/32 and 2002::/16.
			first := []string{"2a00", "2a02", "2a01", "2600", "2400", "2c0f", "2804"}[r.Intn(7)]
			parts := []string{first, fmt.Sprintf("%x", 1+r.Intn(0xfffe))}
			if r.Intn(2) == 0 {
				return strings.Join(parts, ":") + "::" + fmt.Sprintf("%x", 1+r.Intn(0xfffe))
			}
			for len(parts) < 8 {
				parts = append(parts, fmt.Sprintf("%x", r.Intn(0x10000)))
			}
			return strings.Join(parts, ":")
		}
		// First octets with no special-purpose block, a host part that is not a network address.
		first := []int{2, 5, 31, 37, 46, 62, 77, 78, 79, 80, 81, 82, 85, 86, 87, 89, 91, 93, 94, 95, 109, 176, 178, 185, 188, 212, 213, 217}
		return fmt.Sprintf("%d.%d.%d.%d", first[r.Intn(len(first))], r.Intn(256), r.Intn(256), 1+r.Intn(254))
	})
	Register("mac-address", func(r *rand.Rand) string {
		b := make([]byte, 6)
		for i := range b {
			b[i] = byte(r.Intn(256))
		}
		b[0] &^= 1 // unicast
		b[0] |= 2  // locally administered, so no manufacturer's prefix is used
		switch r.Intn(4) {
		case 0:
			return fmt.Sprintf("%02x%02x.%02x%02x.%02x%02x", b[0], b[1], b[2], b[3], b[4], b[5])
		case 1:
			return fmt.Sprintf("%02X-%02X-%02X-%02X-%02X-%02X", b[0], b[1], b[2], b[3], b[4], b[5])
		default:
			return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
		}
	})
}
