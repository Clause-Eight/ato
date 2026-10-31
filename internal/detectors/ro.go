package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Romanian personal numeric code (CNP): S YY MM DD JJ NNN C.
// S gives sex and century, JJ the county (01-48, 51, 52), C a weighted check digit (weights 279146358279, mod 11, 10 becomes 1).
func init() {
	Register(Detector{
		Name:        "ro-cnp",
		Label:       "Romanian personal number (CNP)",
		Country:     "RO",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1-9]\d{12}|` + cnpGroups(" ") + `|` + cnpGroups("-")),
		Normalize:   DigitsOnly,
		Validate:    validCNP,
		Keywords:    []string{"cnp", "cod numeric", "codnumeric", "personal_code", "personalcode", "national_id", "nationalid"},
		Concepts:    []Concept{NationalID},
		Unlikely:    referenceWords(),
		Base:        domain.Medium,
		Validation:  "date, county code and check digit are valid",
	})
}

// cnpGroups are the ways a CNP is printed in groups, with one separator throughout: S YYMMDD JJNNNC (sex, birth
// date, county and number), SYY MMDD JJNNNC, and SYY YMM DDJ JNNNC as phone numbers are grouped.
func cnpGroups(sep string) string {
	return `[1-9]` + sep + `\d{6}` + sep + `\d{6}|[1-9]\d{2}` + sep + `\d{4}` + sep + `\d{6}|[1-9]\d{2}` + sep + `\d{3}` + sep + `\d{3}` + sep + `\d{4}`
}

var cnpWeights = [12]int{2, 7, 9, 1, 4, 6, 3, 5, 8, 2, 7, 9}

func validCNP(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 13 {
		return false
	}
	yy, month, day := d[1]*10+d[2], d[3]*10+d[4], d[5]*10+d[6]
	switch d[0] {
	case 1, 2:
		if !ValidBirthDate(1900+yy, month, day) {
			return false
		}
	case 3, 4:
		if !ValidBirthDate(1800+yy, month, day) {
			return false
		}
	case 5, 6:
		if !ValidBirthDate(2000+yy, month, day) {
			return false
		}
	default:
		// 7, 8 (residents) and 9 (foreigners) do not encode the century.
		if !ValidBirthDate(1900+yy, month, day) && !ValidBirthDate(2000+yy, month, day) {
			return false
		}
	}
	county := d[7]*10 + d[8]
	if !(county >= 1 && county <= 48 || county == 51 || county == 52) {
		return false
	}
	sum := 0
	for i, w := range cnpWeights {
		sum += d[i] * w
	}
	check := sum % 11
	if check == 10 {
		check = 1
	}
	return check == d[12]
}
