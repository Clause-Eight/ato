package synth

import (
	"math/rand"
)

func init() {
	const letters = "TRWAGMYFPDXBNJZSQVHLCKE"
	sep := func(r *rand.Rand) string { return []string{"", "", "-", " "}[r.Intn(4)] }
	Register("es-dni", func(r *rand.Rand) string {
		n := r.Intn(100_000_000)
		return Pad(n, 8) + sep(r) + string(letters[n%23])
	})
	Register("es-nie", func(r *rand.Rand) string {
		prefix, n := r.Intn(3), r.Intn(10_000_000)
		return string("XYZ"[prefix]) + Pad(n, 7) + sep(r) + string(letters[(prefix*10_000_000+n)%23])
	})
}
