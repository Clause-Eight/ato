package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// SIM card number (ICCID, ITU-T E.118): 19 or 20 digits printed on the SIM, starting with 89 (telecommunications),
// then the country calling code and the issuer, with a Luhn check digit at the end. Only European calling codes
// (3 and 4) are matched. A SIM number stays with one subscriber, so it is an online identifier (GDPR recital 30).
// Source: ITU-T Recommendation E.118, https://www.itu.int/rec/T-REC-E.118
func init() {
	Register(Detector{
		Name:        "iccid",
		Label:       "SIM card number (ICCID)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`89[34]\d{16,17}`),
		Validate:    validICCID,
		Keywords:    []string{"iccid", "sim card", "sim number", "sim serial", "simcard", "sim_serial", "sim-nummer", "numéro sim"},
		Base:        domain.Low,
		Validation:  "telecom prefix, European country code and Luhn check digit are valid",
	})
}

func validICCID(s string) bool {
	return (len(s) == 19 || len(s) == 20) && Luhn(s)
}
