package detectors

import (
	"regexp"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Icelandic national identity number (kennitala): DDMMYY-RRCM. RR are two serial digits, C is the check digit
// and M the century (9 for 1900-1999, 0 for 2000-2099, 8 for the 1800s); a birth year after the current one is
// rejected. C is 11 minus the first eight digits
// weighted 3,2,7,6,5,4,3,2, mod 11 (11 is written 0, 10 is never issued). Companies and other legal entities add
// 40 to the day; they are not personal data and are rejected here, as are system numbers (kerfiskennitala)
// that do not start with a date.
// Source: https://en.wikipedia.org/wiki/Kennitala, describing the rules of Registers Iceland (Þjóðskrá).
func init() {
	Register(Detector{
		Name:        "is-kennitala",
		Label:       "Icelandic national identity number (kennitala)",
		Country:     "IS",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`\d{6}-?\d{4}`),
		Normalize:   DigitsOnly,
		Validate:    validKennitala,
		Formatted:   regexp.MustCompile(`^\d{6}-\d{4}$`),
		Keywords:    []string{"kennitala", "kennitölu", "kennitölur", "kennitolu"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "date, century digit and mod-11 check digit are valid",
	})
}

var kennitalaWeights = []int{3, 2, 7, 6, 5, 4, 3, 2}

func validKennitala(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 {
		return false
	}
	century, known := map[int]int{8: 1800, 9: 1900, 0: 2000}[d[9]]
	if !known {
		return false
	}
	year := century + d[4]*10 + d[5]
	if year > time.Now().Year() || !ValidBirthDate(year, d[2]*10+d[3], d[0]*10+d[1]) {
		return false
	}
	sum := 0
	for i, w := range kennitalaWeights {
		sum += d[i] * w
	}
	c := (11 - sum%11) % 11
	return c != 10 && c == d[8]
}
