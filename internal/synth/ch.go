package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("ch-ahv", func(r *rand.Rand) string {
		body := "756" + Digits(r, 9)
		c := (10 - WeightedSum(body, []int{1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3})%10) % 10
		v := body + fmt.Sprint(c)
		if r.Intn(3) == 0 {
			return v
		}
		return v[:3] + "." + v[3:7] + "." + v[7:11] + "." + v[11:]
	})
}
