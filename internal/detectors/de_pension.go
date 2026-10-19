package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// German pension insurance number (Versicherungsnummer, Rentenversicherungsnummer), twelve characters: the area
// number of the issuing carrier (2 digits), the birth date as DDMMYY, the initial letter of the birth surname,
// a serial number (00-49 men, 50-99 women and others) and a check digit. For the check digit the letter becomes
// its two-digit position in the alphabet (A = 01); the twelve digits are multiplied by 2,1,2,5,7,1,2,1,2,1,2,1,
// the digit sums of the products are added, and the sum mod 10 is the check digit.
// Source: Versicherungsnummern-, Kontoführungs- und Versicherungsverlaufsverordnung (VKVV) § 2 and its annex of
// area numbers, https://www.gesetze-im-internet.de/vkvv/__2.html and https://www.gesetze-im-internet.de/vkvv/anlage.html
func init() {
	Register(Detector{
		Name:        "de-pension",
		Label:       "German pension insurance number (Rentenversicherungsnummer)",
		Country:     "DE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{2} ?\d{6} ?[A-Z] ?\d{2} ?\d`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validDEPension,
		Formatted:   regexp.MustCompile(`^\d{2} \d{6} [A-Z] \d{2} ?\d$`),
		Keywords:    []string{"rentenversicherungsnummer", "rentenversicherungs-nr", "rv-nummer", "rvnr", "rv-nr"},
		Concepts:    []Concept{SocialSecurity},
		Base:        domain.Medium,
		Validation:  "area number, birth date and check digit are valid",
	})
}

// deAreaNumbers are the area numbers in the VKVV annex: the regional carriers, the same numbers plus 40 for the
// Deutsche Rentenversicherung Bund, 40 for the Zentrale Zulagenstelle, and the Knappschaft-Bahn-See numbers.
var deAreaNumbers = func() map[int]bool {
	m := map[int]bool{38: true, 39: true, 40: true, 80: true, 81: true, 82: true, 89: true}
	for _, n := range []int{2, 3, 4, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 23, 24, 25, 26, 28, 29} {
		m[n] = true
		m[n+40] = true
	}
	return m
}()

func validDEPension(s string) bool {
	if len(s) != 12 || s[11] < '0' || s[11] > '9' {
		return false
	}
	check, ok := dePensionCheck(s[:11])
	if !ok {
		return false
	}
	d, _ := Digits(s[:8])
	day, month, yy := d[2]*10+d[3], d[4]*10+d[5], d[6]*10+d[7]
	if !deAreaNumbers[d[0]*10+d[1]] || !ValidDate(1900+yy, month, day) && !ValidDate(2000+yy, month, day) {
		return false
	}
	return check == int(s[11]-'0')
}

var dePensionWeights = [12]int{2, 1, 2, 5, 7, 1, 2, 1, 2, 1, 2, 1}

// dePensionCheck computes the check digit of the first eleven characters (eight digits, a letter, two digits).
func dePensionCheck(s string) (int, bool) {
	if len(s) != 11 || s[8] < 'A' || s[8] > 'Z' {
		return 0, false
	}
	head, ok := Digits(s[:8])
	if !ok {
		return 0, false
	}
	serial, ok := Digits(s[9:])
	if !ok {
		return 0, false
	}
	letter := int(s[8]-'A') + 1
	digits := append(head, letter/10, letter%10, serial[0], serial[1])
	sum := 0
	for i, w := range dePensionWeights {
		p := digits[i] * w
		sum += p/10 + p%10
	}
	return sum % 10, true
}
