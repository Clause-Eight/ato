package detectors

import (
	"regexp"
	"strconv"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Polish PESEL: YY MM DD ZZZ X Q. The month carries the century (+80 1800s, +20 2000s, +40 2100s, +60 2200s).
// Check digit: weights 1,3,7,9,1,3,7,9,1,3, then (10 - sum mod 10) mod 10.
// Czech and Slovak birth number (rodné číslo): YYMMDD/SSS[C]. Women add 50 to the month; from 2004 20 or 70 may be added.
// Ten-digit numbers are divisible by 11 (or the first nine leave 10 and the last digit is 0); nine-digit numbers predate 1954.
// Croatian OIB: eleven digits, ISO 7064 MOD 11,10.
func init() {
	Register(Detector{
		Name:        "pl-pesel",
		Label:       "Polish personal number (PESEL)",
		Country:     "PL",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{11}`),
		Validate:    validPESEL,
		Keywords:    []string{"pesel"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "date and check digit are valid",
	})
	Register(Detector{
		Name:        "cz-birth-number",
		Label:       "Czech or Slovak birth number (rodné číslo)",
		Country:     "CZ",
		AlsoIn:      []string{"SK"},
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{6}/?\d{3,4}`),
		Normalize:   DigitsOnly,
		Validate:    validBirthNumber,
		Formatted:   regexp.MustCompile(`^\d{6}/\d{3,4}$`),
		Keywords:    []string{"rodné číslo", "rodne cislo", "rodne_cislo", "rodnecislo", "birth_number", "birthnumber", "rč"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "date and mod-11 check are valid",
	})
	Register(Detector{
		Name:        "hr-oib",
		Label:       "Croatian personal ID (OIB)",
		Country:     "HR",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{11}`),
		Validate:    func(s string) bool { return len(s) == 11 && mod11_10(s) },
		Keywords:    []string{"oib"},
		Concepts:    []Concept{NationalID, TaxID},
		Base:        domain.Low,
		Validation:  "ISO 7064 check digit is valid",
	})
}

var peselWeights = [10]int{1, 3, 7, 9, 1, 3, 7, 9, 1, 3}

func validPESEL(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 {
		return false
	}
	year, month := d[0]*10+d[1], d[2]*10+d[3]
	century := map[int]int{0: 1900, 1: 1900, 2: 2000, 3: 2000, 4: 2100, 5: 2100, 6: 2200, 7: 2200, 8: 1800, 9: 1800}[month/10]
	month -= map[int]int{1900: 0, 2000: 20, 2100: 40, 2200: 60, 1800: 80}[century]
	if !ValidDate(century+year, month, d[4]*10+d[5]) {
		return false
	}
	sum := 0
	for i, w := range peselWeights {
		sum += d[i] * w
	}
	return (10-sum%10)%10 == d[10]
}

func validBirthNumber(s string) bool {
	d, ok := Digits(s)
	if !ok || (len(d) != 9 && len(d) != 10) {
		return false
	}
	yy, month, day := d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]
	switch {
	case month > 70:
		month -= 70
	case month > 50:
		month -= 50
	case month > 20:
		month -= 20
	}
	year := 1900 + yy
	if len(d) == 9 {
		if yy >= 54 {
			return false
		}
	} else if yy < 54 {
		year = 2000 + yy
	}
	if !ValidDate(year, month, day) {
		return false
	}
	if len(d) == 9 {
		return true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return false
	}
	if n%11 == 0 {
		return true
	}
	return (n/10)%11 == 10 && d[9] == 0
}
