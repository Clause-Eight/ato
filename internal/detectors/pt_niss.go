package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Portuguese social security number (NISS, número de identificação da segurança social): eleven digits starting
// with 1 or 2. The last is 9 minus the first ten weighted 29, 23, 19, 17, 13, 11, 7, 5, 3, 2, mod 10. The rule is
// not published by the Segurança Social; it is the one two independent validators implement, and it holds for
// the numbers they were tested on.
// Sources: https://github.com/masterzdran/document-number-validator (NissGenerator.CalculateCheckDigit) and
// https://gist.github.com/dreispt/024dd11c160af58268e2b44019080bbf (controlNISS).
func init() {
	Register(Detector{
		Name:        "pt-niss",
		Label:       "Portuguese social security number (NISS)",
		Country:     "PT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[12]\d{10}`),
		Validate:    validPTNISS,
		Keywords:    []string{"niss", "número de identificação de segurança social", "numero de identificacao de seguranca social", "n.º de segurança social", "nº segurança social", "número da segurança social", "numero da seguranca social"},
		Concepts:    []Concept{SocialSecurity},
		Base:        domain.Low,
		Validation:  "first digit and check digit are valid",
	})
}

var ptNISSWeights = []int{29, 23, 19, 17, 13, 11, 7, 5, 3, 2}

func validPTNISS(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 || d[0] != 1 && d[0] != 2 {
		return false
	}
	sum := 0
	for i, w := range ptNISSWeights {
		sum += w * d[i]
	}
	return 9-sum%10 == d[10]
}
