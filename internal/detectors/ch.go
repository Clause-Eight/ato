package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Swiss social security number (AHV-Nummer, numéro AVS, AHVN13): thirteen digits starting with 756, the ISO
// country code of Switzerland, usually printed 756.XXXX.XXXX.XC. The last digit is the EAN-13 check digit: digits
// in even positions count three times, odd positions once, and the check digit brings the sum to a multiple of 10.
// GS1 assigns Switzerland the barcode prefixes 760-769, so product barcodes do not start with 756.
// Sources: https://fhir.ch/ig/ch-core/StructureDefinition-ch-core-ahvn13-identifier.html (format and rule) and the
// Central Compensation Office (ZAS/CdC) description of the AHVN13 check digit.
func init() {
	Register(Detector{
		Name:        "ch-ahv",
		Label:       "Swiss social security number (AHV/AVS)",
		Country:     "CH",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`756(?:\.\d{4}\.\d{4}\.\d{2}|\d{10})`),
		Normalize:   DigitsOnly,
		Validate:    validAHV,
		Formatted:   regexp.MustCompile(`^756\.\d{4}\.\d{4}\.\d{2}$`),
		Keywords:    []string{"ahv", "avs", "ahvn13", "navs13", "ahv-nr", "ahv nummer", "ahv-nummer", "numéro avs", "no avs", "numero avs", "sozialversicherungsnummer"},
		Concepts:    []Concept{SocialSecurity},
		Base:        domain.Medium,
		Samples:     map[string]bool{"7561234567897": true},
		Validation:  "756 prefix and EAN-13 check digit are valid",
	})
}

func validAHV(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 13 || d[0] != 7 || d[1] != 5 || d[2] != 6 {
		return false
	}
	sum := 0
	for i := 0; i < 12; i++ {
		if i%2 == 1 {
			sum += d[i] * 3
		} else {
			sum += d[i]
		}
	}
	return (10-sum%10)%10 == d[12]
}
