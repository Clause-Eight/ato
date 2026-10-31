package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Dutch education number (onderwijsnummer): given to pupils and students without a citizen service number. Nine
// digits starting with 10; the BSN elfproef sum 9*s0 + 8*s1 + … + 2*s7 - s8 leaves 5 mod 11 instead of 0, so a
// number is never both. It belongs to a pupil, usually a child (GDPR Art. 8).
// Source: DUO, Programma van Eisen BRON, onderwijsnummer; https://nl.wikipedia.org/wiki/Onderwijsnummer
func init() {
	Register(Detector{
		Name:        "nl-onderwijsnummer",
		Label:       "Dutch education number (onderwijsnummer)",
		Country:     "NL",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`10\d{7}`),
		Validate:    validOnderwijsnummer,
		Keywords:    []string{"onderwijsnummer", "onderwijs nummer", "onderwijs_nummer", "leerlingnummer"},
		Base:        domain.Low,
		Validation:  "prefix 10 and the elfproef check are valid",
	})
}

func validOnderwijsnummer(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 || d[0] != 1 || d[1] != 0 {
		return false
	}
	sum := -d[8]
	for i := 0; i < 8; i++ {
		sum += d[i] * (9 - i)
	}
	return (sum%11+11)%11 == 5
}
