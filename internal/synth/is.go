package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("is-kennitala", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			body := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + Pad(20+r.Intn(80), 2)
			c := (11 - WeightedSum(body, []int{3, 2, 7, 6, 5, 4, 3, 2})%11) % 11
			if c == 10 {
				continue
			}
			v := body + fmt.Sprint(c) + fmt.Sprint(year/100%10)
			if r.Intn(2) == 0 {
				return v[:6] + "-" + v[6:]
			}
			return v
		}
	})
}
