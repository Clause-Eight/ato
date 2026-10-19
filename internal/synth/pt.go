package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("pt-nif", func(r *rand.Rand) string {
		prefixes := []string{"1", "2", "2", "3", "45"}
		p := prefixes[r.Intn(len(prefixes))]
		body := p + Digits(r, 8-len(p))
		c := 11 - WeightedSum(body, []int{9, 8, 7, 6, 5, 4, 3, 2})%11
		if c > 9 {
			c = 0
		}
		v := body + fmt.Sprint(c)
		if r.Intn(4) == 0 {
			return v[:3] + " " + v[3:6] + " " + v[6:]
		}
		return v
	})
	Register("pt-citizen-card", func(r *rand.Rand) string {
		civil := Digits(r, 9)
		version := Letters(r, 1, "ZYXW") + Letters(r, 1, "ZYXWVU")
		// Counting from the right, the check digit is weighted 1, the next character 2, and so on alternately.
		values := []int{}
		for _, c := range civil + version {
			if c >= 'A' && c <= 'Z' {
				values = append(values, int(c-'A')+10)
			} else {
				values = append(values, int(c-'0'))
			}
		}
		sum := 0
		for i, v := range values {
			if (len(values)-i)%2 == 1 {
				v *= 2
				if v > 9 {
					v -= 9
				}
			}
			sum += v
		}
		check := fmt.Sprint((10 - sum%10) % 10)
		if r.Intn(2) == 0 {
			return civil[:8] + " " + civil[8:] + " " + version + check
		}
		return civil + version + check
	})
}
