package synth

import "math/rand"

func init() {
	Register("cy-id-card", func(r *rand.Rand) string {
		return Pad(1+r.Intn(9), 1) + Digits(r, 5+r.Intn(2))
	})
	Register("cy-tic", func(r *rand.Rand) string {
		body := string("069"[r.Intn(3)]) + Digits(r, 7)
		// Odd places (first, third, ...) count by the published table, even places by their own value.
		odd := map[byte]int{'0': 1, '1': 0, '2': 5, '3': 7, '4': 9, '5': 13, '6': 15, '7': 17, '8': 19, '9': 21}
		sum := 0
		for i := 0; i < 8; i++ {
			if i%2 == 0 {
				sum += odd[body[i]]
			} else {
				sum += int(body[i] - '0')
			}
		}
		return body + string(rune('A'+sum%26))
	})
}
