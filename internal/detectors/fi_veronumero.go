package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Finnish individual tax number (veronumero): twelve digits, given to everyone who works on a shared
// construction site and printed on their photo ID card there. It is separate from the personal identity code,
// carries no birth date and has no published control digit, so it is reported only with a keyword on its own line.
// Only its own names count: the shared tax-number words, such as ico, also name file types and other numbers, and
// any twelve digits pass.
// Sources: Finnish Tax Administration, Individual tax numbers,
// https://www.vero.fi/en/detailed-guidance/guidance/48791/individual_tax_numbers__instructions_fo/ and
// python-stdnum's fi.veronumero, https://github.com/arthurdejong/python-stdnum/blob/master/stdnum/fi/veronumero.py
func init() {
	Register(Detector{
		Name:          "fi-veronumero",
		Label:         "Finnish individual tax number (veronumero)",
		Country:       "FI",
		OnByDefault:   true,
		Pattern:       regexp.MustCompile(`\d{12}`),
		Validate:      validVeronumero,
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Keywords:      []string{"veronumero", "veronro", "vero numero", "individual tax number", "finnish tax number", "skattenummer"},
		Base:          domain.Low,
		Validation:    "twelve digits; there is no control digit",
	})
}

func validVeronumero(s string) bool {
	_, ok := Digits(s)
	return ok && len(s) == 12
}
