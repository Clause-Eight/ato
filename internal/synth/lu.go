package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("lu-national-id", func(r *rand.Rand) string {
		year, month, day := Date(r)
		body := fmt.Sprintf("%d%s%s%s", year, Pad(month, 2), Pad(day, 2), Digits(r, 3))
		value := body + fmt.Sprint(LuhnDigit(body)) + fmt.Sprint(verhoeffCheck(body))
		if r.Intn(4) == 0 {
			return value[:4] + " " + value[4:8] + " " + value[8:11] + " " + value[11:]
		}
		return value
	})
}

// The Verhoeff tables: multiplication in the dihedral group D5, the position permutations and the inverses.
var (
	verhoeffD = [10][10]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		{1, 2, 3, 4, 0, 6, 7, 8, 9, 5},
		{2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
		{3, 4, 0, 1, 2, 8, 9, 5, 6, 7},
		{4, 0, 1, 2, 3, 9, 5, 6, 7, 8},
		{5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
		{6, 5, 9, 8, 7, 1, 0, 4, 3, 2},
		{7, 6, 5, 9, 8, 2, 1, 0, 4, 3},
		{8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
		{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
	}
	verhoeffP = [8][10]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		{1, 5, 7, 6, 2, 8, 3, 0, 9, 4},
		{5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
		{8, 9, 1, 6, 0, 4, 3, 5, 2, 7},
		{9, 4, 5, 3, 1, 2, 6, 8, 7, 0},
		{4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
		{2, 7, 9, 3, 8, 0, 6, 4, 1, 5},
		{7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
	}
	verhoeffInv = [10]int{0, 4, 3, 2, 1, 5, 6, 7, 8, 9}
)

// verhoeffCheck is the Verhoeff check digit for a string of digits.
func verhoeffCheck(body string) int {
	c := 0
	for i := 0; i < len(body); i++ {
		c = verhoeffD[c][verhoeffP[(i+1)%8][body[len(body)-1-i]-'0']]
	}
	return verhoeffInv[c]
}
