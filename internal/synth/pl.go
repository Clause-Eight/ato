package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("pl-id-card", func(r *rand.Rand) string {
		for {
			series := Letters(r, 3, "")
			if series == "AAA" {
				continue
			}
			rest := Digits(r, 5)
			sum := 0
			for i, w := range []int{7, 3, 1} {
				sum += (int(series[i]-'A') + 10) * w
			}
			sum += WeightedSum(rest, []int{7, 3, 1, 7, 3})
			// The check digit has weight 9: find the digit that makes the total a multiple of ten.
			c := 0
			for (sum+9*c)%10 != 0 {
				c++
			}
			if r.Intn(4) == 0 {
				return fmt.Sprintf("%s %d%s", series, c, rest)
			}
			return fmt.Sprintf("%s%d%s", series, c, rest)
		}
	})
	Register("pl-regon", func(r *rand.Rand) string {
		body := Digits(r, 8)
		regon := body + fmt.Sprint(WeightedSum(body, []int{8, 9, 2, 3, 4, 5, 6, 7})%11%10)
		if r.Intn(3) > 0 {
			return regon
		}
		local := regon + Digits(r, 4)
		return local + fmt.Sprint(WeightedSum(local, []int{2, 4, 8, 5, 0, 9, 7, 3, 6, 1, 2, 4, 8})%11%10)
	})
	Register("pl-nip", func(r *rand.Rand) string {
		for {
			body := Pad(101+r.Intn(899), 3) + Digits(r, 6)
			c := WeightedSum(body, []int{6, 5, 7, 2, 3, 4, 5, 6, 7}) % 11
			if c == 10 {
				continue
			}
			v := body + fmt.Sprint(c)
			switch r.Intn(3) {
			case 0:
				return v[:3] + "-" + v[3:6] + "-" + v[6:8] + "-" + v[8:]
			case 1:
				return v[:3] + "-" + v[3:5] + "-" + v[5:7] + "-" + v[7:]
			}
			return v
		}
	})
}
