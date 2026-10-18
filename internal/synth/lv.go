package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("lv-personal-code", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			body := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + fmt.Sprint(year/100-18) + Digits(r, 3)
			c := ((1101 - WeightedSum(body, []int{1, 6, 3, 7, 9, 10, 5, 8, 4, 2})) % 11)
			if c == 10 {
				continue
			}
			if r.Intn(3) == 0 {
				return body + fmt.Sprint(c)
			}
			return body[:6] + "-" + body[6:] + fmt.Sprint(c)
		}
	})
	Register("lv-personal-code-new", func(r *rand.Rand) string {
		v := "32" + Digits(r, 9)
		if r.Intn(2) == 0 {
			return v
		}
		return v[:6] + "-" + v[6:]
	})
}
