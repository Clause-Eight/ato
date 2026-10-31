package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Romanian electronic identity card (carte electronică de identitate, CEI), issued since 2021: the document
// number on the front ("Nr. document / Document No.") is a two-letter series and seven digits, nine characters
// in all, not the county series and six digits of the older cards (see ro-id-card). The printed number has no
// control digit of its own (the one after it in the machine-readable zone belongs to the MRZ), so a keyword is
// required. Sources: Direcția Județeană de Evidență a Persoanelor Iași, Generalități CEI, with the specimen,
// https://www.djep-iasi.ro/wp-content/uploads/2025/08/GENERALITATI-CEI.pdf and Europa Liberă România, "Tot ce
// trebuie să știi despre noua carte electronică de identitate", https://romania.europalibera.org/a/cartea-electronica-de-identitate/33354995.html
// (the series is two letters and seven characters).
func init() {
	Register(Detector{
		Name:         "ro-cei",
		Label:        "Romanian electronic identity card number (carte electronică de identitate, CEI)",
		Country:      "RO",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[A-Z]{2} ?\d{7}`),
		Normalize:    func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:     validROCEI,
		Needs:        Needs{Check: func(line string) bool { return capsThenDigit(line, 2, 1) }},
		NeedsKeyword: true,
		Keywords: []string{"carte electronică de identitate", "carte electronica de identitate", "cartea electronică de identitate",
			"cartea electronica de identitate", "nr. document", "nr document", "număr document", "numar document", "seria cei", "serie cei", "nr. cei", "nr cei"},
		Concepts:   []Concept{IDCard},
		Base:       domain.Low,
		Validation: "two-letter series and seven digits; there is no control digit",
	})
}

func validROCEI(s string) bool {
	if len(s) != 9 || s[0] < 'A' || s[0] > 'Z' || s[1] < 'A' || s[1] > 'Z' {
		return false
	}
	_, ok := Digits(s[2:])
	return ok
}
