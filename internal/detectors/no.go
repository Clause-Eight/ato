package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Norwegian national identity number (fødselsnummer): DDMMYY, a three-digit individual number and two check
// digits. The first is 11 minus the first nine digits weighted 3,7,6,1,8,9,4,5,2, mod 11; the second is 11 minus
// the first ten weighted 5,4,3,2,7,6,5,4,3,2, mod 11; 11 is written 0 and 10 is never issued. The individual
// number gives the century: 000-499 1900-1999, 500-749 with YY >= 54 1854-1899, 500-999 with YY < 40 2000-2039,
// 900-999 with YY >= 40 1940-1999. A D-number adds 4 to the first digit of the day; a health-service H-number adds
// 4 to the first digit of the month. Synthetic test numbers (Skatteetaten's Tenor adds 80 to the month, NHN's
// test population 65) belong to nobody and are rejected, as is the system that starts in 2032 with other checks.
// Sources: https://www.skatteetaten.no/rettskilder/type/handboker/folkeregisterhandboken/gjeldende/kapittel2-registreringspliktige-personer-identifikasjonsnummer/ID-2-2.001/
// (structure, D-number) and https://no.wikipedia.org/wiki/Fødselsnummer (check digits, individual number ranges, H-number).
func init() {
	Register(Detector{
		Name:        "no-fnr",
		Label:       "Norwegian national identity number (fødselsnummer, D-nummer)",
		Country:     "NO",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`\d{6} ?\d{5}`),
		Normalize:   DigitsOnly,
		Validate:    validNOFnr,
		Formatted:   regexp.MustCompile(`^\d{6} \d{5}$`),
		Keywords:    []string{"fødselsnummer", "fodselsnummer", "fnr", "d-nummer", "dnummer", "f.nr", "personnr"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "date, century and both mod-11 check digits are valid",
	})
}

var (
	noFnrWeights1 = []int{3, 7, 6, 1, 8, 9, 4, 5, 2}
	noFnrWeights2 = []int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
)

func validNOFnr(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 {
		return false
	}
	day, month, yy := d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]
	switch {
	case day > 40 && month > 40:
		return false
	case day > 40:
		day -= 40
	case month > 40:
		month -= 40
	}
	century := noFnrCentury(d[6]*100+d[7]*10+d[8], yy)
	if century == 0 || !ValidDate(century+yy, month, day) {
		return false
	}
	return noFnrCheck(d, noFnrWeights1) && noFnrCheck(d, noFnrWeights2)
}

func noFnrCentury(individual, yy int) int {
	switch {
	case individual < 500:
		return 1900
	case individual < 750 && yy >= 54:
		return 1800
	case yy < 40:
		return 2000
	case individual >= 900:
		return 1900
	}
	return 0
}

// noFnrCheck reports whether the digit after the weighted ones is 11 minus their sum mod 11 (11 is written 0).
func noFnrCheck(d []int, weights []int) bool {
	sum := 0
	for i, w := range weights {
		sum += d[i] * w
	}
	c := (11 - sum%11) % 11
	return c != 10 && c == d[len(weights)]
}
