package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// German tax ID (steuerliche Identifikationsnummer, IdNr, § 139b AO): eleven digits, the first never 0. In the
// first ten digits exactly one digit occurs twice or (since 2016) three times and every other digit at most once;
// a digit that occurs three times never fills three consecutive positions. Digit eleven is the ISO 7064
// MOD 11,10 check digit of the first ten.
// Source: Bayerisches Landesamt für Steuern, "Prüfung der Steuer- und Steueridentifikationsnummer", section 2.2,
// https://download.elster.de/download/schnittstellen/Pruefung_der_Steuer_und_Steueridentifikatsnummer.pdf
// The samples are the worked examples printed there, taken from the ITZ Bund concept.
func init() {
	Register(Detector{
		Name:        "de-tax-id",
		Label:       "German tax ID (steuerliche Identifikationsnummer)",
		Country:     "DE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1-9]\d(?: ?\d{3}){3}`),
		Normalize:   DigitsOnly,
		Validate:    validDETaxID,
		Formatted:   regexp.MustCompile(`^\d{2} \d{3} \d{3} \d{3}$`),
		Keywords:    []string{"steuerliche identifikationsnummer", "steuer-identifikationsnummer", "steueridentifikationsnummer", "steuer-idnr", "steueridnr", "steuer-id", "steuerid", "idnr"},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Samples:     map[string]bool{"86095742719": true, "47036892816": true, "65929970489": true, "57549285017": true, "25768131411": true},
		Validation:  "digit rule and ISO 7064 check digit are valid",
	})
}

func validDETaxID(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 || d[0] == 0 {
		return false
	}
	var count [10]int
	for _, x := range d[:10] {
		count[x]++
	}
	repeated := -1
	for digit, n := range count {
		switch {
		case n <= 1:
		case n > 3 || repeated >= 0:
			return false
		default:
			repeated = digit
		}
	}
	if repeated < 0 {
		return false
	}
	for i := 0; i+2 < 10; i++ {
		if d[i] == repeated && d[i+1] == repeated && d[i+2] == repeated {
			return false
		}
	}
	return mod11_10(s)
}
