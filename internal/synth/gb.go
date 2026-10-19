package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("gb-nino", func(r *rand.Rand) string {
		const first, second = "ABCEGHJKLMNOPRSTWXYZ", "ABCEGHJKLMNPRSTWXYZ"
		for {
			prefix := Letters(r, 1, first) + Letters(r, 1, second)
			if strings.Contains(" BG GB KN NK NT TN ZZ ", " "+prefix+" ") {
				continue
			}
			digits, suffix := Digits(r, 6), Letters(r, 1, "ABCD")
			if r.Intn(2) == 0 {
				return prefix + " " + digits[:2] + " " + digits[2:4] + " " + digits[4:] + " " + suffix
			}
			return prefix + digits + suffix
		}
	})
	Register("gb-nhs", func(r *rand.Rand) string {
		for {
			body := fmt.Sprint([]int{4, 6, 7}[r.Intn(3)]) + Digits(r, 8)
			c := 11 - WeightedSum(body, []int{10, 9, 8, 7, 6, 5, 4, 3, 2})%11
			if c == 10 {
				continue
			}
			if c == 11 {
				c = 0
			}
			v := body + fmt.Sprint(c)
			if r.Intn(2) == 0 {
				return v[:3] + " " + v[3:6] + " " + v[6:]
			}
			return v
		}
	})
}
