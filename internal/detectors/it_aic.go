package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Italian medicine authorisation code (codice AIC, Autorizzazione all'Immissione in Commercio): the number AIFA
// gives each authorised pack, printed on it and on prescriptions. In its decimal form it is nine digits starting
// with 0; the last is a check digit over the first eight, which are weighted 1 and 2 alternately from the left,
// the digits of each product added together and the total taken modulo 10 (python-stdnum, module stdnum.it.aic,
// https://arthurdejong.org/python-stdnum/doc/stdnum.it.aic). The six-character base-32 form of the same number
// is not matched.
//
// A pack code says which medicine a person takes only in a record about them, so the detector needs a medicine
// word and a person in the same record, like the ICD and ATC codes (health.go, ADR 14).
func init() {
	Register(Detector{
		Name:        "it-aic",
		Label:       "Italian medicine code (AIC) in a record about a person (health data, GDPR Art. 9)",
		Country:     "IT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`0\d{8}`),
		Validate:    validAIC,
		Keywords: []string{"aic", "codice aic", "minsan", "farmaco", "farmaci", "medicinale", "medicine", "medication", "drug",
			"prescrizione", "ricetta"},
		Base:         domain.Medium,
		Validation:   "nine digits starting with 0, with a valid check digit",
		NeedsKeyword: true,
		NeedsPerson:  true,
		Special:      domain.SpecialHealth,
	})
}

func validAIC(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 || d[0] != 0 {
		return false
	}
	sum := 0
	for i, x := range d[:8] {
		x *= 1 + i%2
		sum += x/10 + x%10
	}
	return sum%10 == d[8]
}
