package detectors

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/synth"
)

// genGTIN builds a synthetic GTIN of the given length; the check digit is computed from the left, the way GS1
// tables print it, rather than from the right as ValidGTIN does.
func genGTIN(r *rand.Rand, length int) string {
	body := synth.Digits(r, length-1)
	weights := make([]int, length-1)
	for i := range weights {
		weights[i] = 1
		if (length-1-i)%2 == 1 {
			weights[i] = 3
		}
	}
	return body + fmt.Sprint((10-synth.WeightedSum(body, weights)%10)%10)
}

func TestValidGTIN(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for _, length := range []int{8, 12, 13, 14} {
		for range 50 {
			v := genGTIN(r, length)
			if !ValidGTIN(v) {
				t.Fatalf("a valid GTIN of %d digits was rejected", length)
			}
			wrong := v[:length-1] + fmt.Sprint((int(v[length-1]-'0')+1)%10)
			if ValidGTIN(wrong) {
				t.Fatalf("a GTIN of %d digits with a wrong check digit was accepted", length)
			}
		}
	}
	for _, v := range []string{genGTIN(r, 10), genGTIN(r, 16), "", "1234567X"} {
		if ValidGTIN(v) {
			t.Errorf("a value of %d characters is not a GTIN", len(v))
		}
	}
}
