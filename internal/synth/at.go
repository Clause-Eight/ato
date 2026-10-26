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
	Register("at-tax-number", func(r *rand.Rand) string {
		body := Pad(1+r.Intn(98), 2) + Digits(r, 6)
		// Weights 1 and 2 alternate from the left; a two-digit product counts as the sum of its digits.
		sum := 0
		for i := 0; i < 8; i++ {
			p := int(body[i]-'0') * (1 + i%2)
			sum += p/10 + p%10
		}
		v := body + fmt.Sprint((10-sum%10)%10)
		switch r.Intn(3) {
		case 0:
			return v[:2] + "-" + v[2:5] + "/" + v[5:]
		case 1:
			return v[:2] + " " + v[2:5] + "/" + v[5:]
		}
		return v
	})
}
