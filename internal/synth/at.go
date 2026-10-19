package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("at-social-insurance", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			if r.Intn(20) == 0 {
				month = 13 + r.Intn(3)
			}
			serial := Pad(100+r.Intn(900), 3)
			date := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2)
			c := WeightedSum(serial+date, []int{3, 7, 9, 5, 8, 4, 2, 1, 6}) % 11
			if c == 10 {
				continue
			}
			sep := ""
			if r.Intn(2) == 0 {
				sep = " "
			}
			return serial + fmt.Sprint(c) + sep + date
		}
	})
}
