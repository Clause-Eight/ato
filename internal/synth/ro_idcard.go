package synth

import "math/rand"

func init() {
	Register("ro-id-card", func(r *rand.Rand) string {
		series := []string{"RX", "RT", "KX", "CJ", "AX", "TM", "IZ", "BV", "PH", "SV"}[r.Intn(10)]
		switch r.Intn(3) {
		case 0:
			return series + Digits(r, 6)
		case 1:
			return series + " " + Digits(r, 6)
		}
		return series + " nr. " + Digits(r, 6)
	})
}
