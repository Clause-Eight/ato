package synth

import (
	"math/rand"
	"strconv"
)

func init() {
	Register("be-national-register", func(r *rand.Rand) string {
		year, month, day := Date(r)
		switch r.Intn(10) {
		case 0:
			month += 40
		case 1:
			month, day = 0, 0
		}
		body := Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Pad(1+r.Intn(998), 3)
		n, _ := strconv.Atoi(body)
		if year >= 2000 {
			n += 2_000_000_000
		}
		check := Pad(97-n%97, 2)
		if r.Intn(2) == 0 {
			return body + check
		}
		return body[:2] + "." + body[2:4] + "." + body[4:6] + "-" + body[6:] + "." + check
	})
}
