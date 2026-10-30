package engine

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/synth"
)

// gtinCard is a synthetic 13-digit number that is both a valid Maestro card (Luhn) and a valid GTIN-13. The GTIN
// check is computed here, independently of the detector.
func gtinCard(t *testing.T, seed int64) string {
	t.Helper()
	r := rand.New(rand.NewSource(seed))
	weights := []int{1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3}
	for range 10000 {
		body := "59" + synth.Digits(r, 10)
		check := (10 - synth.WeightedSum(body, weights)%10) % 10
		if synth.LuhnDigit(body) == check {
			return body + fmt.Sprint(check)
		}
	}
	t.Fatal("no number valid as both card and GTIN")
	return ""
}

func TestGTINProductCodesAreNotCards(t *testing.T) {
	cases := []struct {
		name   string
		format string
		found  bool
		want   domain.Confidence
	}{
		{"gtin label", "gtin: %s", false, 0},
		{"ean label", `{"ean13": "%s"}`, false, 0},
		{"barcode label", "barcode=%s", false, 0},
		{"upc label", "upc %s", false, 0},
		{"sku lowers only", "sku: %s", true, domain.Low},
		{"no label", "value %s", true, domain.Medium},
		{"card keyword", "card number %s", true, domain.High},
	}
	for seed := int64(1); seed <= 10; seed++ {
		v := gtinCard(t, seed)
		for _, c := range cases {
			fs := scanWith(t, "card", fmt.Sprintf(c.format, v))
			if !c.found {
				if len(fs) != 0 {
					t.Errorf("%s, seed %d: a valid GTIN next to its label must not be a card", c.name, seed)
				}
				continue
			}
			if len(fs) != 1 || fs[0].Confidence != c.want {
				t.Errorf("%s, seed %d: want one finding at %s, got %d", c.name, seed, c.want, len(fs))
			}
		}
	}
}

func TestGTINWordsOnlyLowerCardsThatAreNotGTINs(t *testing.T) {
	for seed := int64(1); seed <= 10; seed++ {
		v := synthValue(t, "card", seed)
		fs := scanWith(t, "card", "ean: "+v)
		if len(fs) != 1 || fs[0].Confidence != domain.Low {
			t.Errorf("seed %d: a card that is no GTIN is lowered, not dropped: %d findings", seed, len(fs))
		}
	}
}
