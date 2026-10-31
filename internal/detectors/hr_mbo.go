package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Croatian health insurance number (matični broj osigurane osobe, MBO): nine digits that the Croatian Health
// Insurance Fund (HZZO) gives each insured person and prints on the health insurance card. It is used next to the
// OIB in health records, which makes it health-related data under GDPR Art. 9 in most uses. HZZO does not
// publish a control rule for it, so only the length is known and a keyword on its own line is required.
// Source: HZZO, Upute za razmjenu podataka na portalu (the insured person's number has nine digits),
// http://www.cezih.hr/dokumenti/Upute_za_razmjenu_podataka_na_portalu_01022017.pdf
func init() {
	Register(Detector{
		Name:          "hr-mbo",
		Label:         "Croatian health insurance number (MBO)",
		Country:       "HR",
		OnByDefault:   true,
		Pattern:       regexp.MustCompile(`\d{9}`),
		Validate:      validMBO,
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Keywords: []string{"mbo", "matični broj osigurane osobe", "maticni broj osigurane osobe", "matični broj osiguranika",
			"maticni broj osiguranika"},
		Concepts:   []Concept{HealthInsurance},
		Base:       domain.Low,
		Validation: "nine digits, not all the same; there is no published control digit",
	})
}

func validMBO(s string) bool {
	if _, ok := Digits(s); !ok || len(s) != 9 {
		return false
	}
	return s != strings.Repeat(s[:1], 9)
}
