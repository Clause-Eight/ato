package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Lithuanian personal code (asmens kodas): G YYMMDD SSS C, the same structure and check digit as the Estonian
// code (see ee.go): G 1-6 gives century and sex, C is mod 11 with weights 1-9,1, then 3-9,1,2,3, then 0.
// Codes with an unknown month or day (00) and codes starting with 9 exist but are rare and are not reported.
// Source: standard LST 1185-91 "Asmens kodas. Sudėtis ir struktūra", described at https://lt.wikipedia.org/wiki/Asmens_kodas
func init() {
	Register(Detector{
		Name:        "lt-asmens-kodas",
		Label:       "Lithuanian personal code (asmens kodas)",
		Country:     "LT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1-6]\d{10}`),
		Validate:    validBalticCode,
		Keywords:    []string{"asmens kodas", "asmens kodą", "asmens kodo", "asmenskodas"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "birth date, century digit and check digit are valid",
	})
}
