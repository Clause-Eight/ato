package synth

import "math/rand"

func init() {
	Register("mt-id-card", func(r *rand.Rand) string {
		year, _, _ := Date(r)
		letter := Letters(r, 1, "MG")
		if year >= 2000 {
			letter = Letters(r, 1, "LH")
		}
		v := Digits(r, 5) + Pad(year%100, 2) + letter
		if r.Intn(4) == 0 {
			for len(v) > 4 && v[0] == '0' {
				v = v[1:]
			}
		}
		return v
	})
}
