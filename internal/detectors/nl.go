package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Dutch citizen service number (burgerservicenummer, BSN; formerly the sofinummer): nine digits s0..s8 for which
// 9*s0 + 8*s1 + 7*s2 + 6*s3 + 5*s4 + 4*s5 + 3*s6 + 2*s7 - 1*s8 is divisible by 11 (the elfproef). Eight-digit
// numbers carry an implied leading zero; only the nine-digit form is matched here, as it is usually written.
// Source: RvIG, Logisch Ontwerp BSN 2025.Q4, paragraph II.2.3, element Burgerservicenummer,
// https://rvig.nl/sites/default/files/2025-11/Logisch%20Ontwerp%20BSN%202025.Q4.pdf
func init() {
	Register(Detector{
		Name:        "nl-bsn",
		Label:       "Dutch citizen service number (BSN)",
		Country:     "NL",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{9}|\d{4}\.\d{2}\.\d{3}`),
		Normalize:   DigitsOnly,
		Validate:    validBSN,
		Formatted:   regexp.MustCompile(`^\d{4}\.\d{2}\.\d{3}$`),
		Keywords:    []string{"bsn", "burgerservicenummer", "sofinummer", "sofi-nummer"},
		Concepts:    []Concept{NationalID, TaxID},
		Base:        domain.Low,
		Validation:  "elfproef check is valid",
	})
}

func validBSN(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 {
		return false
	}
	sum := -d[8]
	nonzero := d[8] != 0
	for i := 0; i < 8; i++ {
		sum += d[i] * (9 - i)
		nonzero = nonzero || d[i] != 0
	}
	return nonzero && sum%11 == 0
}
