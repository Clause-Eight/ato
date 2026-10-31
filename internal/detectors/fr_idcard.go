package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// French national identity card number (numéro de la carte nationale d'identité). The card issued from 1995 to
// 2021 has twelve characters: the year (two digits) and month of issue, three letters or digits for the issuing
// department, and a five-digit number given in order of application. The card issued since 2021 has nine letters
// and digits. The check digit that follows the number in the machine-readable zone is not printed with it, so
// there is no check here. Sources: Microsoft Purview, "France national ID card (CNI)" (format),
// https://learn.microsoft.com/en-us/purview/sit-defn-france-national-id-card and
// https://fr.wikipedia.org/wiki/Carte_nationale_d%27identit%C3%A9_en_France (the twelve-character layout).
//
// A nine-character value must hold at least one letter and one digit, so that a bare number or a word in
// capitals is not taken for a 2021 card. Both forms look like many other values, so the line itself must name
// the card.
func init() {
	Register(Detector{
		Name:          "fr-id-card",
		Label:         "French national identity card number (CNI)",
		Country:       "FR",
		OnByDefault:   true,
		Pattern:       regexp.MustCompile(`[0-9]{4}[0-9A-Z]{3}[0-9]{5}|[0-9A-Z]{9}`),
		Validate:      validFRIDCard,
		Keywords:      []string{"numéro de carte d'identité", "numéro de la carte d'identité", "numéro cni", "n° cni", "no cni"},
		Concepts:      []Concept{IDCard},
		Base:          domain.Low,
		Validation:    "month of issue is valid (1995 card) or letters and digits are mixed (2021 card); there is no control digit",
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Needs:         Needs{Check: frNamesCard},
	})
}

func validFRIDCard(s string) bool {
	switch len(s) {
	case 12:
		d, ok := Digits(s[:4])
		if !ok || d[2]*10+d[3] < 1 || d[2]*10+d[3] > 12 {
			return false
		}
		_, ok = Digits(s[7:])
		return ok
	case 9:
		return strings.ContainsAny(s, "0123456789") && strings.Trim(s, "0123456789") != ""
	}
	return false
}

var frCardWords = []string{"identit", "Identit", "IDENTIT", "cni", "Cni", "CNI"}

// frNamesCard is the prefilter: a line that does not name the card cannot hold a keyword.
func frNamesCard(line string) bool {
	for _, w := range frCardWords {
		if strings.Contains(line, w) {
			return true
		}
	}
	return false
}
