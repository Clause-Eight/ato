package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("gr-amka", func(r *rand.Rand) string {
		year, month, day := Date(r)
		body := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + Digits(r, 4)
		return body + fmt.Sprint(LuhnDigit(body))
	})
	Register("gr-afm", func(r *rand.Rand) string {
		body := Pad(r.Intn(2), 1) + Digits(r, 7)
		if body == "00000000" {
			body = "01000000"
		}
		return body + fmt.Sprint(WeightedSum(body, []int{256, 128, 64, 32, 16, 8, 4, 2})%11%10)
	})
}
