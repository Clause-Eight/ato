package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Hungarian identity card number (személyi igazolvány okmányazonosító): six digits and two letters. It names the
// card, not the holder, and is not the personal identification number stored in the chip. The source gives the
// characters but not their order, so both orders are accepted. There is no check digit. Source:
// https://en.wikipedia.org/wiki/Hungarian_identity_card ("six numbers and two letters").
func init() {
	Register(Detector{
		Name:         "hu-id-card",
		Label:        "Hungarian identity card number (személyi igazolvány)",
		Country:      "HU",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[0-9]{6}[A-Z]{2}|[A-Z]{2}[0-9]{6}`),
		Keywords:     []string{"okmányazonosító", "személyi igazolvány száma", "személyi igazolvány szám", "személyi igazolványszám", "szig szám", "szigszám"},
		Concepts:     []Concept{IDCard},
		Base:         domain.Low,
		Validation:   "six digits and two letters; there is no control digit",
		NeedsKeyword: true,
		Needs:        Needs{Check: func(line string) bool { return digitThenCapital(line) || capsThenDigit(line, 2, 0) }},
	})
}
