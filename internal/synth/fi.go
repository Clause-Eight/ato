package synth

import (
	"math/rand"
	"strconv"
)

func init() {
	Register("fi-hetu", func(r *rand.Rand) string {
		year, month, day := Date(r)
		signs := "-YXWVU"
		if year >= 2000 {
			signs = "ABCDEF"
		}
		sign := signs[0]
		if r.Intn(5) == 0 {
			sign = signs[r.Intn(len(signs))]
		}
		body := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + Pad(2+r.Intn(898), 3)
		n, _ := strconv.Atoi(body)
		return body[:6] + string(sign) + body[6:] + string("0123456789ABCDEFHJKLMNPRSTUVWXY"[n%31])
	})
}
