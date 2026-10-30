package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Latvian personal code (personas kods), 11 digits usually written DDMMYY-CNNNZ.
// Codes issued until 30 June 2017: DDMMYY is the birth date, C the century (0 1800s, 1 1900s, 2 2000s), NNN a
// serial number and Z a check digit, (1101 - the first ten digits weighted 1,6,3,7,9,10,5,8,4,2) mod 11; a
// result of 10 is never issued.
// Codes issued since 1 July 2017 start with 32 and the other nine digits are random, with no check digit, so
// they are a second detector that needs a keyword. Day 32 never starts an old code, so the two never overlap.
// Sources: https://learn.microsoft.com/purview/sit-defn-latvia-personal-code (format),
// https://lvportals.lv/skaidrojumi/288359-no-julija-saks-pieskirt-jaunos-personas-kodus-2017 (new codes),
// https://laacz.lv/2006/11/25/pk-parbaudes-algoritms (check digit; no official publication was found).
func init() {
	Register(Detector{
		Name:        "lv-personal-code",
		Label:       "Latvian personal code (personas kods)",
		Country:     "LV",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{6}-?\d{5}`),
		Normalize:   DigitsOnly,
		Validate:    validLVPersonalCode,
		Formatted:   regexp.MustCompile(`^\d{6}-\d{5}$`),
		Keywords:    lvKeywords,
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "birth date, century digit and check digit are valid",
	})
	Register(Detector{
		Name:         "lv-personal-code-new",
		Label:        "Latvian personal code without birth date (personas kods, since 2017)",
		Country:      "LV",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`32\d{4}-?\d{5}`),
		Normalize:    DigitsOnly,
		Validate:     validLVPersonalCodeNew,
		Formatted:    regexp.MustCompile(`^32\d{4}-\d{5}$`),
		Keywords:     lvKeywords,
		Concepts:     []Concept{NationalID},
		Base:         domain.Low,
		NeedsKeyword: true,
		Validation:   "eleven digits starting with 32",
	})
}

var lvKeywords = []string{"personas kods", "personas kodu", "personas koda", "personaskods"}

var lvWeights = [10]int{1, 6, 3, 7, 9, 10, 5, 8, 4, 2}

func validLVPersonalCode(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 || d[6] > 2 {
		return false
	}
	if !ValidDate(1800+d[6]*100+d[4]*10+d[5], d[2]*10+d[3], d[0]*10+d[1]) {
		return false
	}
	sum := 0
	for i, w := range lvWeights {
		sum += d[i] * w
	}
	c := (1101 - sum) % 11
	return c < 10 && c == d[10]
}

func validLVPersonalCodeNew(s string) bool {
	_, ok := Digits(s)
	return ok && len(s) == 11 && s[:2] == "32"
}
