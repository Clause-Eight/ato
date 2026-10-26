package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("si-emso", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			register := 50
			if r.Intn(5) == 0 {
				register += 1 + r.Intn(9)
			}
			body := Pad(day, 2) + Pad(month, 2) + Pad(year%1000, 3) + Pad(register, 2) + Digits(r, 3)
			rem := WeightedSum(body, []int{7, 6, 5, 4, 3, 2, 7, 6, 5, 4, 3, 2}) % 11
			if rem == 1 {
				continue
			}
			return body + fmt.Sprint((11-rem)%11)
		}
	})
	Register("si-tax-number", func(r *rand.Rand) string {
		for {
			body := Pad(1000000+r.Intn(9000000), 7)
			c := 11 - WeightedSum(body, []int{8, 7, 6, 5, 4, 3, 2})%11
			if c == 11 {
				continue
			}
			if c == 10 {
				c = 0
			}
			return body + fmt.Sprint(c)
		}
	})
}
