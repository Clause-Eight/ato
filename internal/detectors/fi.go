package detectors

import (
	"regexp"
	"strconv"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Finnish personal identity code (henkilötunnus, HETU): DDMMYY C ZZZ Q. The separator C gives the century:
// + for the 1800s, - or Y, X, W, V, U for the 1900s, A or B, C, D, E, F for the 2000s (the letters other than A
// were added in 2023). ZZZ is the individual number (002-899, 900-999 for temporary codes). Q is the nine-digit
// number DDMMYYZZZ modulo 31, written as 0-9 then A-Y without G, I, O and Q.
// Sources: Government Decree 690/2022 amending section 2 of decree 128/2010,
// https://finlex.fi/api/media/statute/17253/mainPdf/main.pdf, and https://stat.fi/meta/kas/hetu.html
func init() {
	Register(Detector{
		Name:        "fi-hetu",
		Label:       "Finnish personal identity code (henkilötunnus)",
		Country:     "FI",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{6}[-+A-FU-Y]\d{3}[0-9A-FHJ-NPR-Y]`),
		Validate:    validHETU,
		Keywords:    []string{"hetu", "personbeteckning"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Medium,
		Validation:  "birth date, century sign and check character are valid",
	})
}

const hetuCheckChars = "0123456789ABCDEFHJKLMNPRSTUVWXY"

func validHETU(s string) bool {
	if len(s) != 11 {
		return false
	}
	var century int
	switch c := s[6]; {
	case c == '+':
		century = 1800
	case c == '-' || c >= 'U' && c <= 'Y':
		century = 1900
	case c >= 'A' && c <= 'F':
		century = 2000
	default:
		return false
	}
	digits := s[:6] + s[7:10]
	d, ok := Digits(digits)
	if !ok {
		return false
	}
	if !ValidDate(century+d[4]*10+d[5], d[2]*10+d[3], d[0]*10+d[1]) || d[6]*100+d[7]*10+d[8] < 2 {
		return false
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return false
	}
	return hetuCheckChars[n%31] == s[10]
}
