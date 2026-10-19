package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("dk-cpr", func(r *rand.Rand) string {
		year, month, day := Date(r)
		var seventh int
		switch {
		case year >= 2000:
			seventh = []int{4, 5, 6, 7, 8, 9}[r.Intn(6)]
		case year >= 1937:
			seventh = []int{0, 1, 2, 3, 4, 9}[r.Intn(6)]
		default:
			seventh = r.Intn(4)
		}
		sep := ""
		if r.Intn(3) > 0 {
			sep = "-"
		}
		return Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + sep + fmt.Sprint(seventh) + Digits(r, 3)
	})
}
