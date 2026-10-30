package detectors

import (
	"regexp"
	"time"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Malta identity card number: up to seven digits and a letter. The first five digits are the birth
// registration number at the Public Registry and the last two the year of registration; leading zeros are
// often left out. The letter gives the holder's group: M Maltese and G Gozitan births registered 1900-1999,
// L and H after 2000, B and Z 1800-1899, P Maltese citizens born abroad, A foreign residence permit holders.
// The same number is the tax number of Maltese nationals. There is no check digit; L and H numbers must end
// in a year that has already come. Source: Identity Malta, Identity Cards Unit,
// https://identita.gov.mt/identity-cards-unit/useful-info/
func init() {
	Register(Detector{
		Name:         "mt-id-card",
		Label:        "Malta identity card number",
		Country:      "MT",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`\d{3,7} ?[MGAPLHBZ]`),
		Normalize:    mtRemoveSpace,
		Validate:     validMTIDCard,
		Formatted:    regexp.MustCompile(`^\d{7}[MGAPLHBZ]$`),
		Keywords:     []string{"karta tal-identità", "karta tal-identita", "numru tal-identità", "numru tal-identita", "maltese id", "malta id", "identity malta"},
		Concepts:     []Concept{IDCard, NationalID, TaxID},
		Base:         domain.Low,
		Validation:   "letter and registration year fit; there is no control digit",
		NeedsKeyword: true,
		Needs:        Needs{Check: mtDigitThenLetter},
	})
}

func mtRemoveSpace(s string) string {
	if len(s) > 1 && s[len(s)-2] == ' ' {
		return s[:len(s)-2] + s[len(s)-1:]
	}
	return s
}

func validMTIDCard(s string) bool {
	if len(s) < 4 || len(s) > 8 {
		return false
	}
	d, ok := Digits(s[:len(s)-1])
	if !ok {
		return false
	}
	switch s[len(s)-1] {
	case 'L', 'H':
		return d[len(d)-2]*10+d[len(d)-1] <= time.Now().Year()%100
	case 'M', 'G', 'A', 'P', 'B', 'Z':
		return true
	}
	return false
}

// mtDigitThenLetter is the prefilter: a digit followed, after at most one space, by one of the card letters.
func mtDigitThenLetter(line string) bool {
	for i := 0; i+1 < len(line); i++ {
		if line[i] < '0' || line[i] > '9' {
			continue
		}
		j := i + 1
		if line[j] == ' ' && j+1 < len(line) {
			j++
		}
		switch line[j] {
		case 'M', 'G', 'A', 'P', 'L', 'H', 'B', 'Z':
			return true
		}
	}
	return false
}
