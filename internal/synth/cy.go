package synth

import "math/rand"

func init() {
	Register("cy-id-card", func(r *rand.Rand) string {
		return Pad(1+r.Intn(9), 1) + Digits(r, 5+r.Intn(2))
	})
}
