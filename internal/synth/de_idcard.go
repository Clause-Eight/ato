package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("de-id-card", func(r *rand.Rand) string {
		body := Letters(r, 1, "LMNPRTVWXY") + Letters(r, 8, "0123456789CFGHJKLMNPRTVWXYZ")
		return body + fmt.Sprint(mrzDigit(body))
	})
}

// mrzDigit is the ICAO 9303 check digit: digits as values, A-Z as 10-35, weights 7, 3, 1.
func mrzDigit(s string) int {
	weights := []int{7, 3, 1}
	sum := 0
	for i, c := range s {
		v := int(c - '0')
		if c >= 'A' {
			v = int(c-'A') + 10
		}
		sum += v * weights[i%3]
	}
	return sum % 10
}
