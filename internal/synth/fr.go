package synth

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

func init() {
	Register("fr-nir", func(r *rand.Rand) string {
		year, month, _ := Date(r)
		var place, key string
		switch r.Intn(6) {
		case 0:
			place = "97" + fmt.Sprint(1+r.Intn(6)) + Pad(1+r.Intn(30), 2)
		case 1:
			place = "99" + fmt.Sprint(1+r.Intn(5)) + Pad(1+r.Intn(99), 2)
		case 2:
			place = []string{"2A", "2B"}[r.Intn(2)] + Pad(1+r.Intn(400), 3)
		default:
			place = Pad(1+r.Intn(95), 2) + Pad(1+r.Intn(999), 3)
		}
		body := fmt.Sprint(1+r.Intn(2)) + Pad(year%100, 2) + Pad(month, 2) + place + Pad(1+r.Intn(999), 3)
		numeric := strings.NewReplacer("2A", "19", "2B", "18").Replace(body)
		n, _ := strconv.ParseInt(numeric, 10, 64)
		key = Pad(int(97-n%97), 2)
		if r.Intn(2) == 0 {
			return body + key
		}
		return strings.Join([]string{body[:1], body[1:3], body[3:5], body[5:7], body[7:10], body[10:13], key}, " ")
	})
	Register("fr-spi", func(r *rand.Rand) string {
		body := fmt.Sprint(r.Intn(4)) + Digits(r, 9)
		n, _ := strconv.ParseInt(body, 10, 64)
		v := body + Pad(int(n%511), 3)
		if r.Intn(2) == 0 {
			return v
		}
		return strings.Join([]string{v[:2], v[2:4], v[4:7], v[7:10], v[10:]}, " ")
	})
}
