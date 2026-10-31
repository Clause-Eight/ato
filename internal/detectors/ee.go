package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Estonian personal code (isikukood): G YYMMDD SSS C. G gives century and sex (1-2 1800s, 3-4 1900s, 5-6 2000s;
// 7-8 for the 2100s are not issued yet and are rejected). C: weights 1-9,1 over the first ten digits, sum mod 11;
// a remainder of 10 is retried with weights 3-9,1,2,3, and a second 10 gives 0.
// Source: standard EVS 585:2007 "Isikukood. Struktuur", described at https://et.wikipedia.org/wiki/Isikukood
func init() {
	Register(Detector{
		Name:        "ee-isikukood",
		Label:       "Estonian personal code (isikukood)",
		Country:     "EE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1-6]\d{10}`),
		Validate:    validBalticCode,
		Keywords:    []string{"isikukood", "isikukoodi", "isikukoodiga"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "birth date, century digit and check digit are valid",
	})
}

var (
	balticWeights1 = [10]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 1}
	balticWeights2 = [10]int{3, 4, 5, 6, 7, 8, 9, 1, 2, 3}
)

// validBalticCode checks the Estonian and Lithuanian personal codes, which share structure and check digit.
func validBalticCode(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 || d[0] < 1 || d[0] > 6 {
		return false
	}
	year := 1800 + (d[0]-1)/2*100 + d[1]*10 + d[2]
	if !ValidDate(year, d[3]*10+d[4], d[5]*10+d[6]) {
		return false
	}
	return balticCheckDigit(d) == d[10]
}

func balticCheckDigit(d []int) int {
	for _, w := range [][10]int{balticWeights1, balticWeights2} {
		sum := 0
		for i, x := range w {
			sum += d[i] * x
		}
		if r := sum % 11; r != 10 {
			return r
		}
	}
	return 0
}
