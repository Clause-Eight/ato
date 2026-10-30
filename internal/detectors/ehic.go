package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/internal/domain"
)

// European Health Insurance Card number: the logical identification number of the card (field 8). Decision S2 of
// the Administrative Commission (OJ C 106, 24.4.2010, p. 26, Annex I section 3.5.4,
// https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32010D0424(09)) makes it 20 characters: a 10-character
// issuer identifier under EN 1867, then a 10-digit serial with leading zeros. EN 1867 issuer numbers follow
// ISO/IEC 7812-1, where the health care range is 80 followed by the three-digit ISO 3166-1 numeric country code
// (https://en.wikipedia.org/wiki/ISO/IEC_7812). The detector accepts 20 digits that start with 80 and the code of
// an EHIC country (EU, EEA, Switzerland, and the UK's GHIC), optionally grouped as 10+10 or 5+5+10.
//
// Limitations: the card number has no public check digit; the decision lets a member state without chip cards
// register issuers ad hoc instead of under EN 1867, and those cards are missed. The personal identification
// number (field 6) is the national number and is left to the national detectors. Off by default and only
// reported with a keyword.
func init() {
	Register(Detector{
		Name:         "ehic",
		Label:        "European Health Insurance Card number",
		Country:      domain.CrossBorder,
		Pattern:      regexp.MustCompile(`80\d{3} ?\d{5} ?\d{10}`),
		Normalize:    DigitsOnly,
		Validate:     validEHIC,
		Keywords:     []string{"ehic", "ghic", "ceam", "ceass", "cead", "ekuz", "ekvk", "tse", "ekaa", "european health insurance card", "european health card"},
		Concepts:     []Concept{HealthInsurance},
		Base:         domain.Low,
		Validation:   "20 digits with the health-card issuer prefix (80 and a member state's country code)",
		NeedsKeyword: true,
	})
}

// ehicCountries are the ISO 3166-1 numeric codes of the countries that issue the card.
var ehicCountries = map[string]bool{
	"040": true, "056": true, "100": true, "191": true, "196": true, "203": true, "208": true, "233": true,
	"246": true, "250": true, "276": true, "300": true, "348": true, "352": true, "372": true, "380": true,
	"428": true, "438": true, "440": true, "442": true, "470": true, "528": true, "578": true, "616": true,
	"620": true, "642": true, "703": true, "705": true, "724": true, "752": true, "756": true, "826": true,
}

func validEHIC(s string) bool {
	if _, ok := Digits(s); !ok || len(s) != 20 || s[:2] != "80" {
		return false
	}
	return ehicCountries[s[2:5]]
}
