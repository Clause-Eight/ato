package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("nl-bsn", func(r *rand.Rand) string {
		for {
			body := Digits(r, 8)
			c := WeightedSum(body, []int{9, 8, 7, 6, 5, 4, 3, 2}) % 11
			if c == 10 || body == "00000000" {
				continue
			}
			v := body + fmt.Sprint(c)
			if r.Intn(4) == 0 {
				return v[:4] + "." + v[4:6] + "." + v[6:]
			}
			return v
		}
	})
}
