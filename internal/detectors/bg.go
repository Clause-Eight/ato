package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Bulgarian unified civil number (EGN): YY MM DD RRR C. Months 21-32 mean the 1800s, 41-52 the 2000s.
// Check digit: weights 2,4,8,5,10,9,7,3,6, mod 11, 10 becomes 0.
// Personal number of a foreigner (LNCh): ten digits, weights 21,19,17,13,11,9,7,3,1, mod 10.
func init() {
	Register(Detector{
		Name:        "bg-egn",
		Label:       "Bulgarian personal number (EGN)",
		Country:     "BG",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{10}`),
		Validate:    validEGN,
		Keywords:    []string{"egn", "егн", "personal_number", "personalnumber", "national_id", "nationalid"},
		Base:        domain.Low,
		Validation:  "date and check digit are valid",
	})
	Register(Detector{
		Name:        "bg-lnch",
		Label:       "Bulgarian foreigner number (LNCh)",
		Country:     "BG",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{10}`),
		Validate:    validLNCh,
		Keywords:    []string{"lnch", "лнч"},
		Base:        domain.Low,
		Validation:  "check digit is valid",
	})
}

var egnWeights = [9]int{2, 4, 8, 5, 10, 9, 7, 3, 6}

func validEGN(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 {
		return false
	}
	year, month := d[0]*10+d[1], d[2]*10+d[3]
	switch {
	case month >= 41 && month <= 52:
		year, month = 2000+year, month-40
	case month >= 21 && month <= 32:
		year, month = 1800+year, month-20
	default:
		year += 1900
	}
	if !ValidDate(year, month, d[4]*10+d[5]) {
		return false
	}
	sum := 0
	for i, w := range egnWeights {
		sum += d[i] * w
	}
	check := sum % 11
	if check == 10 {
		check = 0
	}
	return check == d[9]
}

var lnchWeights = [9]int{21, 19, 17, 13, 11, 9, 7, 3, 1}

func validLNCh(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 {
		return false
	}
	sum := 0
	for i, w := range lnchWeights {
		sum += d[i] * w
	}
	return sum%10 == d[9]
}
