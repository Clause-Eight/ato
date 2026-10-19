package synth

import (
	"fmt"
	"math/rand"
	"strconv"
)

func init() {
	Register("pl-pesel", func(r *rand.Rand) string {
		year, month, day := Date(r)
		if year >= 2000 {
			month += 20
		}
		body := Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Digits(r, 4)
		return body + fmt.Sprint((10-WeightedSum(body, []int{1, 3, 7, 9, 1, 3, 7, 9, 1, 3})%10)%10)
	})
	Register("cz-birth-number", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			if year < 1954 {
				continue
			}
			if r.Intn(2) == 1 {
				month += 50
			}
			body := Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Digits(r, 3)
			n, _ := strconv.ParseInt(body, 10, 64)
			c := (11 - n*10%11) % 11
			if c == 10 {
				continue
			}
			return body[:6] + "/" + body[6:] + fmt.Sprint(c)
		}
	})
	Register("hr-oib", func(r *rand.Rand) string {
		body := Digits(r, 10)
		return body + fmt.Sprint(Mod1110Digit(body))
	})
}
