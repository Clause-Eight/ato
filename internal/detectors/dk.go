package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Danish CPR number (CPR-nummer): DDMMYY-SSSS. The seventh digit together with the year gives the century:
// 0-3 is 1900-1999; 4 and 9 are 2000-2036 for years 00-36 and 1937-1999 otherwise; 5-8 are 2000-2057 for
// years 00-57 and 1858-1899 otherwise. Numbers issued since 1 October 2007 need not pass the old modulus-11
// test, so only the date is validated and a keyword is required.
// Sources: https://cpr.dk/cpr-systemet/opbygning-af-cpr-nummeret and https://da.wikipedia.org/wiki/CPR-nummer
func init() {
	Register(Detector{
		Name:         "dk-cpr",
		Label:        "Danish personal number (CPR-nummer)",
		Country:      "DK",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`\d{6}-?\d{4}`),
		Normalize:    DigitsOnly,
		Validate:     validCPR,
		Formatted:    regexp.MustCompile(`^\d{6}-\d{4}$`),
		Keywords:     []string{"cpr", "cpr-nummer", "cprnummer", "cpr-nr"},
		Concepts:     []Concept{NationalID},
		Base:         domain.Low,
		NeedsKeyword: true,
		Validation:   "birth date and century digit are valid",
	})
}

func validCPR(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 {
		return false
	}
	day, month, yy := d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]
	return ValidBirthDate(cprCentury(d[6], yy)+yy, month, day)
}

func cprCentury(seventh, yy int) int {
	switch {
	case seventh <= 3:
		return 1900
	case seventh == 4 || seventh == 9:
		if yy <= 36 {
			return 2000
		}
		return 1900
	default:
		if yy <= 57 {
			return 2000
		}
		return 1800
	}
}
