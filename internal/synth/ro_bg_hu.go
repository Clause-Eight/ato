package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("ro-cnp", func(r *rand.Rand) string {
		year, month, day := Date(r)
		sex := map[int]int{19: 1, 20: 5}[year/100] + r.Intn(2)
		counties := []int{1, 7, 12, 22, 40, 46, 51, 52}
		body := fmt.Sprintf("%d%s%s%s%s%s", sex, Pad(year%100, 2), Pad(month, 2), Pad(day, 2), Pad(counties[r.Intn(len(counties))], 2), Digits(r, 3))
		c := WeightedSum(body, []int{2, 7, 9, 1, 4, 6, 3, 5, 8, 2, 7, 9}) % 11
		if c == 10 {
			c = 1
		}
		return body + fmt.Sprint(c)
	})
	Register("bg-egn", func(r *rand.Rand) string {
		year, month, day := Date(r)
		if year >= 2000 {
			month += 40
		}
		body := Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Digits(r, 3)
		return body + fmt.Sprint(WeightedSum(body, []int{2, 4, 8, 5, 10, 9, 7, 3, 6})%11%10)
	})
	Register("bg-lnch", func(r *rand.Rand) string {
		body := Digits(r, 9)
		return body + fmt.Sprint(WeightedSum(body, []int{21, 19, 17, 13, 11, 9, 7, 3, 1})%10)
	})
	Register("hu-taj", func(r *rand.Rand) string {
		body := Digits(r, 8)
		return body + fmt.Sprint(WeightedSum(body, []int{3, 7, 3, 7, 3, 7, 3, 7})%10)
	})
	Register("hu-tax-id", func(r *rand.Rand) string {
		for {
			body := "8" + Pad(1000+r.Intn(50000), 5) + Digits(r, 3)
			if c := WeightedSum(body, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}) % 11; c != 10 {
				return body + fmt.Sprint(c)
			}
		}
	})
}
