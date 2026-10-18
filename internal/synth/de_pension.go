package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	areas := []int{2, 3, 4, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 23, 24, 25, 26, 28, 29, 38, 39, 40, 42, 53, 65, 69, 80, 81, 82, 89}
	Register("de-pension", func(r *rand.Rand) string {
		year, month, day := Date(r)
		letter := byte('A' + r.Intn(26))
		head := Pad(areas[r.Intn(len(areas))], 2) + Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2)
		serial := Pad(r.Intn(100), 2)
		digits := head + Pad(int(letter-'A')+1, 2) + serial
		sum := 0
		for i, w := range []int{2, 1, 2, 5, 7, 1, 2, 1, 2, 1, 2, 1} {
			p := int(digits[i]-'0') * w
			sum += p/10 + p%10
		}
		check := fmt.Sprint(sum % 10)
		if r.Intn(2) == 0 {
			return head[:2] + " " + head[2:] + " " + string(letter) + " " + serial + check
		}
		return head + string(letter) + serial + check
	})
}
