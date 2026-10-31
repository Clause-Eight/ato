package synth

import "math/rand"

// eircodeKeys are some of the 139 routing keys, from the public list of Eircode routing areas.
var eircodeKeys = []string{"A41", "A92", "C15", "D01", "D02", "D6W", "D15", "E91", "F92", "H91", "K78", "N91", "P31", "R95",
	"T12", "T23", "V94", "W91", "X91", "Y35"}

func init() {
	Register("ie-eircode", func(r *rand.Rand) string {
		const unique = "0123456789ACDEFHKNPRTVWXY"
		id := Letters(r, 4, unique)
		for id[1:] == id[:3] { // four of one character is a placeholder
			id = Letters(r, 4, unique)
		}
		key := eircodeKeys[r.Intn(len(eircodeKeys))]
		if r.Intn(4) == 0 {
			return key + id
		}
		return key + " " + id
	})
}
