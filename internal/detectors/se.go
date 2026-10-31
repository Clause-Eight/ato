package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Swedish personal identity number (personnummer): YYMMDD-NNNC, with + instead of - from the year the person
// turns 100, or YYYYMMDDNNNC in registers. NNN is the birth number (001-999). C is the modulus-10 check with
// weights 2 and 1 over the ten-digit form (the Luhn method). Coordination numbers (samordningsnummer) add 60
// to the day.
// Sources: Skatteverket SKV 704 "Personnummer", https://docs.swedenconnect.se/technical-framework/mirror/skv/skv704-8.pdf,
// and https://skatteverket.se/samordningsnummer
func init() {
	Register(Detector{
		Name:        "se-personnummer",
		Label:       "Swedish personal identity number (personnummer)",
		Country:     "SE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:\d{2})?\d{6}[-+]?\d{4}`),
		Normalize:   DigitsOnly,
		Validate:    validPersonnummer,
		Formatted:   regexp.MustCompile(`^\d{6}[-+]\d{4}$|^(?:18|19|20)\d{6}-\d{4}$`),
		Keywords:    []string{"personnummer", "personnr", "samordningsnummer"},
		Concepts:    []Concept{NationalID, TaxID},
		Base:        domain.Low,
		Validation:  "birth date and Luhn check digit are valid",
	})
}

func validPersonnummer(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 && len(d) != 12 {
		return false
	}
	centuries := []int{1900, 2000}
	if len(d) == 12 {
		c := d[0]*10 + d[1]
		if c < 18 || c > 20 {
			return false
		}
		centuries = []int{c * 100}
		d = d[2:]
	}
	yy, month, day := d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]
	if day > 60 {
		day -= 60
	}
	dated := false
	for _, c := range centuries {
		dated = dated || ValidBirthDate(c+yy, month, day)
	}
	if !dated || d[6]*100+d[7]*10+d[8] == 0 {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		p := d[i] * (2 - i%2)
		sum += p/10 + p%10
	}
	return (10-sum%10)%10 == d[9]
}
