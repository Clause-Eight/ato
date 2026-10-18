package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("de-tax-id", func(r *rand.Rand) string {
		body := deTaxIDBody(r)
		v := body + fmt.Sprint(Mod1110Digit(body))
		if r.Intn(3) == 0 {
			return v[:2] + " " + v[2:5] + " " + v[5:8] + " " + v[8:]
		}
		return v
	})
}

// deTaxIDBody returns ten digits, the first not 0, in which one digit appears twice or three times and every
// other digit at most once; three equal digits are never all adjacent.
func deTaxIDBody(r *rand.Rand) string {
	for {
		perm := r.Perm(10)
		repeated := perm[0]
		var digits []int
		if r.Intn(2) == 0 {
			digits = append(perm[1:9], repeated, repeated)
		} else {
			digits = append(perm[1:8], repeated, repeated, repeated)
		}
		r.Shuffle(len(digits), func(i, j int) { digits[i], digits[j] = digits[j], digits[i] })
		var b strings.Builder
		for _, d := range digits {
			b.WriteByte(byte('0' + d))
		}
		s := b.String()
		triple := strings.Repeat(string(rune('0'+repeated)), 3)
		if s[0] != '0' && !strings.Contains(s, triple) {
			return s
		}
	}
}
