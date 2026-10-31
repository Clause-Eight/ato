package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Hungarian personal identification number (személyi azonosító, the "személyi szám"): eleven digits. The first
// gives sex and century: 1 and 2 for people born in the 1900s, 3 and 4 from 2000, 5 and 6 for people born in the
// 1900s who were not Hungarian citizens at birth, 7 and 8 for the 1800s. Digits 2-7 are the birth date YYMMDD,
// 8-10 a serial within the day, and the last is the check digit: the first ten digits weighted by their position
// (1 to 10) mod 11 for people born before 1997, weighted 10 down to 1 from 1997. Serials that would give 10 are
// never issued.
// Sources: Act LXVI of 1992, annex, https://static.valasztas.hu/parval2006/hu/02/1992lxvi13parmell.pdf (structure
// and the position-weighted check) and https://hu.wikipedia.org/wiki/Személyi_azonosító (the reversed weights
// from 1997).
func init() {
	Register(Detector{
		Name:        "hu-personal-id",
		Label:       "Hungarian personal identification number (személyi azonosító)",
		Country:     "HU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1-8][ -]?\d{6}[ -]?\d{4}`),
		Normalize:   DigitsOnly,
		Validate:    validHUPersonalID,
		Formatted:   regexp.MustCompile(`^\d[ -]\d{6}[ -]\d{4}$`),
		Keywords:    []string{"személyi azonosító", "szemelyi azonosito", "személyi szám", "szemelyi szam", "személyi azonosító jel", "szig"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "sex and century digit, birth date and mod-11 check digit are valid",
	})
}

func validHUPersonalID(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 {
		return false
	}
	var year int
	switch d[0] {
	case 1, 2, 5, 6:
		year = 1900
	case 3, 4:
		year = 2000
	case 7, 8:
		year = 1800
	default:
		return false
	}
	year += d[1]*10 + d[2]
	month, day := d[3]*10+d[4], d[5]*10+d[6]
	if !ValidBirthDate(year, month, day) {
		return false
	}
	sum := 0
	for i := 0; i < 10; i++ {
		if year >= 1997 {
			sum += d[i] * (10 - i)
		} else {
			sum += d[i] * (i + 1)
		}
	}
	return sum%11 == d[10]
}
