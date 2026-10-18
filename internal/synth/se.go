package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("se-personnummer", func(r *rand.Rand) string {
		year, month, day := Date(r)
		if r.Intn(5) == 0 {
			day += 60
		}
		body := Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Pad(1+r.Intn(999), 3)
		value := body + fmt.Sprint(LuhnDigit(body))
		switch r.Intn(4) {
		case 0:
			return fmt.Sprint(year/100) + value
		case 1:
			return value
		default:
			return value[:6] + "-" + value[6:]
		}
	})
}
