package synth

import "math/rand"

func init() {
	Register("ie-ppsn", func(r *rand.Rand) string {
		body := Digits(r, 7)
		sum := WeightedSum(body, []int{8, 7, 6, 5, 4, 3, 2})
		second := ""
		switch r.Intn(3) {
		case 0:
			second = "A"
			sum += 9
		case 1:
			second = "W"
		}
		letters := "WABCDEFGHIJKLMNOPQRSTUV"
		return body + string(letters[sum%23]) + second
	})
}
