package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Belgian electronic identity card number (kaartnummer, numéro de carte): twelve digits, printed 999-9999999-99.
// The last two are the first ten mod 97, with 97 when the remainder is 0. The number belongs to the card, not to the
// holder, and is not the national register number printed on the same card. Source: python-stdnum, stdnum.be.eid,
// https://arthurdejong.org/python-stdnum/doc/2.1/stdnum.be.eid
//
// Belgian bank account numbers from before IBAN have the same layout and the same check, so the card needs a
// keyword, and a value on a line about an account is left to the account.
func init() {
	Register(Detector{
		Name:         "be-id-card",
		Label:        "Belgian eID card number",
		Country:      "BE",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`\d{3}-?\d{7}-?\d{2}`),
		Normalize:    DigitsOnly,
		Validate:     validBEIDCard,
		Formatted:    regexp.MustCompile(`^\d{3}-\d{7}-\d{2}$`),
		Keywords:     []string{"eid-kaartnummer", "kaartnummer identiteitskaart", "numéro de la carte d'identité électronique", "numéro de carte eid", "carte eid", "eid-kaart", "eid card number", "belgian eid"},
		Concepts:     []Concept{IDCard},
		Base:         domain.Low,
		Validation:   "mod-97 check digits are valid",
		NeedsKeyword: true,
		Rival: &Rival{
			Words: []string{"rekeningnummer", "rekening", "bankrekening", "numéro de compte", "compte bancaire", "account number", "bank account"},
			Valid: validBEIDCard,
		},
	})
}

func validBEIDCard(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 12 {
		return false
	}
	rem := 0
	for _, x := range d[:10] {
		rem = (rem*10 + x) % 97
	}
	if rem == 0 {
		rem = 97
	}
	return rem == d[10]*10+d[11]
}
