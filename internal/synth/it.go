package synth

import (
	"math/rand"
	"strings"
)

func init() {
	Register("it-fiscal-code", func(r *rand.Rand) string {
		year, month, day := Date(r)
		if r.Intn(2) == 1 {
			day += 40
		}
		municipality := Letters(r, 1, "ABCDEFGHILMZ") + Digits(r, 3)
		code := []byte(Letters(r, 6, "") + Pad(year%100, 2) + string("ABCDEHLMPRST"[month-1]) + Pad(day, 2) + municipality)
		if r.Intn(4) == 0 {
			numeric := []int{14, 13, 12, 10, 9, 7, 6}
			for _, i := range numeric[:1+r.Intn(len(numeric))] {
				code[i] = "LMNPQRSTUV"[code[i]-'0']
			}
		}
		return string(code) + string(fiscalCheck(string(code)))
	})
}

// fiscalCheck computes the Italian codice fiscale check letter from its own odd-position table.
func fiscalCheck(body string) byte {
	const alnum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	odd := []int{1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 2, 4, 18, 20, 11, 3, 6, 8, 12, 14, 16, 10, 22, 25, 24, 23}
	even := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}
	sum := 0
	for i := 0; i < 15; i++ {
		k := strings.IndexByte(alnum, body[i])
		if (i+1)%2 == 1 {
			sum += odd[k]
		} else {
			sum += even[k]
		}
	}
	return byte('A' + sum%26)
}
