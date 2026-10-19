package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("no-fnr", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			var individual int
			switch {
			case year >= 2000:
				individual = 500 + r.Intn(500)
			case year >= 1940 && r.Intn(4) == 0:
				individual = 900 + r.Intn(100)
			default:
				individual = r.Intn(500)
			}
			switch r.Intn(5) {
			case 0:
				day += 40 // D-number
			case 1:
				month += 40 // H-number
			}
			body := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + Pad(individual, 3)
			k1 := (11 - WeightedSum(body, []int{3, 7, 6, 1, 8, 9, 4, 5, 2})%11) % 11
			if k1 == 10 {
				continue
			}
			body += fmt.Sprint(k1)
			k2 := (11 - WeightedSum(body, []int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2})%11) % 11
			if k2 == 10 {
				continue
			}
			v := body + fmt.Sprint(k2)
			if r.Intn(4) == 0 {
				return v[:6] + " " + v[6:]
			}
			return v
		}
	})
}
