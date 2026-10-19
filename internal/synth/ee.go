package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("ee-isikukood", balticCode)
}

// balticCode builds an Estonian or Lithuanian personal code: century-and-sex digit, date, serial, mod-11 check.
func balticCode(r *rand.Rand) string {
	year, month, day := Date(r)
	g := (year/100-18)*2 + 1 + r.Intn(2)
	body := fmt.Sprint(g) + Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Digits(r, 3)
	c := WeightedSum(body, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 1}) % 11
	if c == 10 {
		c = WeightedSum(body, []int{3, 4, 5, 6, 7, 8, 9, 1, 2, 3}) % 11 % 10
	}
	return body + fmt.Sprint(c)
}
