package synth

import (
	"math/rand"
	"strconv"
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
	Register("es-nss", func(r *rand.Rand) string {
		provinces := []int{1, 8, 15, 28, 29, 41, 46, 48, 52, 66}
		p := provinces[r.Intn(len(provinces))]
		seq := r.Intn(100_000_000)
		whole, _ := strconv.ParseInt(Pad(p, 2)+Pad(seq, 8), 10, 64)
		if seq < 10_000_000 {
			// Older numbers: the province is followed by a seven-digit sequence.
			whole, _ = strconv.ParseInt(Pad(p, 2)+Pad(seq, 7), 10, 64)
		}
		s := []string{"", " ", "/"}[r.Intn(3)]
		return Pad(p, 2) + s + Pad(seq, 8) + s + Pad(int(whole%97), 2)
	})
}
