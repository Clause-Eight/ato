package synth

import (
	"math/rand"
	"strings"
)

// Generators for identifiers that have a layout but no control digit. Each follows the published layout
// without calling the detector.
func init() {
	Register("fi-veronumero", func(r *rand.Rand) string {
		return Digits(r, 12)
	})
	Register("de-stnr", func(r *rand.Rand) string {
		// Each layout: the regional form with F, B, U and P for the tax office, district, serial and final digit,
		// and the federal form that puts the state code in front.
		layouts := [][2]string{
			{"FF/BBB/UUUUP", "28FF0BBBUUUUP"}, {"FFF/BBB/UUUUP", "9FFF0BBBUUUUP"}, {"FF/BBB/UUUUP", "11FF0BBBUUUUP"},
			{"0FF/BBB/UUUUP", "30FF0BBBUUUUP"}, {"0FF/BBB/UUUUP", "26FF0BBBUUUUP"}, {"FFF/BBBB/UUUP", "5FFF0BBBBUUUP"},
			{"1FF/BBB/UUUUP", "41FF0BBBUUUUP"}, {"2FF/BBB/UUUUP", "32FF0BBBUUUUP"}, {"FF/BBB/UUUUP", "21FF0BBBUUUUP"},
		}
		l := layouts[r.Intn(len(layouts))]
		fill := func(layout string) string {
			var b strings.Builder
			for _, c := range layout {
				if strings.ContainsRune("FBUP", c) {
					b.WriteString(Digits(r, 1))
				} else {
					b.WriteRune(c)
				}
			}
			return b.String()
		}
		switch r.Intn(3) {
		case 0:
			return fill(l[1])
		case 1:
			return strings.ReplaceAll(fill(l[0]), "/", "")
		}
		return fill(l[0])
	})
	Register("li-peid", func(r *rand.Rand) string {
		v := string(rune('1'+r.Intn(9))) + Digits(r, 3+r.Intn(9))
		if len(v) < 11 && r.Intn(4) == 0 {
			v = strings.Repeat("0", 11-len(v)) + v
		}
		return v
	})
	Register("ro-cei", func(r *rand.Rand) string {
		v := Letters(r, 2, "") + Digits(r, 7)
		if r.Intn(5) == 0 {
			return v[:2] + " " + v[2:]
		}
		return v
	})
	Register("hr-mbo", func(r *rand.Rand) string {
		return string(rune('1'+r.Intn(9))) + Digits(r, 8)
	})
	Register("imsi", func(r *rand.Rand) string {
		// Mobile country codes from ITU-T E.212, with a two-digit network code.
		mccs := []string{"202", "204", "208", "214", "216", "219", "222", "226", "228", "232", "234", "260", "262", "268", "284"}
		return mccs[r.Intn(len(mccs))] + Digits(r, 12)
	})
}
