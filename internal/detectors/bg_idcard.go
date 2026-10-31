package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Bulgarian identity card number (номер на лична карта): nine digits on the front of the card. It names the card,
// not the holder, and is not the EGN. There is no check digit. Source:
// https://en.wikipedia.org/wiki/Bulgarian_identity_card ("the unique 9-digit ID number of the card").
//
// The card issued since June 2024 may use letters as well; no public source gives its layout, so it is not
// covered. A bare nine-digit number is too common to trust a keyword on the line before, so the line itself must
// name the card.
func init() {
	Register(Detector{
		Name:          "bg-id-card",
		Label:         "Bulgarian identity card number (лична карта)",
		Country:       "BG",
		OnByDefault:   true,
		Pattern:       regexp.MustCompile(`\d{9}`),
		Keywords:      []string{"номер на лична карта", "номер на личната карта", "лична карта №", "лична карта номер"},
		Concepts:      []Concept{IDCard},
		Base:          domain.Low,
		Validation:    "nine digits on a line naming the card; there is no control digit",
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Needs:         Needs{Check: bgNamesCard},
	})
}

var bgCardWords = []string{"лична", "Лична", "ЛИЧНА"}

// bgNamesCard is the prefilter: a line that does not name the card cannot hold a keyword.
func bgNamesCard(line string) bool {
	for _, w := range bgCardWords {
		if strings.Contains(line, w) {
			return true
		}
	}
	return false
}
