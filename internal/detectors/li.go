package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Liechtenstein person identification number (Personenidentifikationsnummer, PEID): the tax number of people
// and entities registered in Liechtenstein, a number of up to twelve digits that may be written with leading
// zeros. Without them it has four to twelve digits. There is no control digit, and a short number is anything,
// so it is reported only with one of its own names on its line and is off by default (Liechtenstein is outside the
// EU). Sources: OECD, Liechtenstein TIN sheet,
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/LIECHTENSTEIN%20TIN.pdf
// and python-stdnum's li.peid, https://github.com/arthurdejong/python-stdnum/blob/master/stdnum/li/peid.py
func init() {
	Register(Detector{
		Name:          "li-peid",
		Label:         "Liechtenstein person identification number (PEID)",
		Country:       "LI",
		OnByDefault:   false,
		Pattern:       regexp.MustCompile(`\d{4,12}`),
		Validate:      validPEID,
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Keywords:      []string{"peid", "personenidentifikationsnummer", "personen-identifikationsnummer", "pe-id"},
		Base:          domain.Low,
		Validation:    "four to twelve digits without leading zeros; there is no control digit",
	})
}

func validPEID(s string) bool {
	if _, ok := Digits(s); !ok {
		return false
	}
	n := len(strings.TrimLeft(s, "0"))
	return n >= 4 && n <= 12
}
