package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("imei", func(r *rand.Rand) string {
		body := []string{"35", "86", "01", "99"}[r.Intn(4)] + Digits(r, 12)
		imei := body + fmt.Sprint(LuhnDigit(body))
		switch r.Intn(3) {
		case 0:
			return imei[:2] + "-" + imei[2:8] + "-" + imei[8:14] + "-" + imei[14:]
		case 1:
			return imei[:2] + " " + imei[2:8] + " " + imei[8:14] + " " + imei[14:]
		}
		return imei
	})
	Register("advertising-id", func(r *rand.Rand) string {
		const hex = "0123456789abcdef"
		var b strings.Builder
		for i := 0; i < 32; i++ {
			c := hex[r.Intn(16)]
			switch i {
			case 12:
				c = '4' // random (version 4) UUID, as both platforms issue
			case 16:
				c = "89ab"[r.Intn(4)]
			}
			b.WriteByte(c)
			if i == 7 || i == 11 || i == 15 || i == 19 {
				b.WriteByte('-')
			}
		}
		id := b.String()
		if r.Intn(2) == 0 {
			return strings.ToUpper(id) // IDFA is usually printed in upper case
		}
		return id
	})
}
