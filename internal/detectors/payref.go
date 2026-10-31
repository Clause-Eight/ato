package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Payment references link a payment to the person who made it.
//
// ISO 11649 creditor reference: RF, two check digits and up to 21 letters and digits, printed in groups of four.
// Moved to the end with letters as 10-35, RF and the check digits make the whole number 1 mod 97 (ISO 7064 MOD
// 97-10), as for an IBAN. Source: ISO 11649:2009 and https://en.wikipedia.org/wiki/Creditor_Reference
//
// Swiss QR reference (and the older ESR/ISR reference): 27 digits printed as two digits and five groups of five;
// the last is the recursive mod-10 check digit of the first 26 (table 0,9,4,6,8,2,7,1,3,5).
// Source: Swiss Implementation Guidelines for the QR-bill, section 3.6.2,
// https://www.six-group.com/en/products-services/banking-services/payment-standardization/standards/qr-bill.html
func init() {
	Register(Detector{
		Name:        "rf-creditor-reference",
		Label:       "Creditor payment reference (ISO 11649 RF)",
		Country:     "EU",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`RF\d{2}(?: ?[0-9A-Z]{1,4}){1,6}`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validRF,
		TrimGroups:  true,
		Keywords:    []string{"creditor reference", "payment reference", "structured reference", "referencia", "zahlungsreferenz", "référence de paiement", "betalingskenmerk", "viitenumero"},
		Base:        domain.Medium,
		Validation:  "RF prefix and mod-97 check digits are valid",
	})
	Register(Detector{
		Name:        "ch-qr-reference",
		Label:       "Swiss QR or ESR payment reference",
		Country:     "CH",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`\d{2}(?: ?\d{5}){5}`),
		Normalize:   DigitsOnly,
		Validate:    validCHQRReference,
		Formatted:   regexp.MustCompile(`^\d{2}(?: \d{5}){5}$`),
		Keywords:    []string{"qr-referenz", "qr reference", "esr", "isr", "bvr", "referenznummer", "numéro de référence"},
		Base:        domain.Low,
		Validation:  "recursive mod-10 check digit is valid",
	})
}

func validRF(s string) bool {
	if len(s) < 5 || len(s) > 25 || !strings.HasPrefix(s, "RF") {
		return false
	}
	rem, ok := mod97(s[4:] + s[:4])
	return ok && rem == 1
}

// mod97 is the remainder of the number written by s, with letters A-Z read as 10-35 (ISO 7064 MOD 97-10).
func mod97(s string) (int, bool) {
	rem := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			rem = (rem*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			rem = (rem*100 + int(c-'A'+10)) % 97
		default:
			return 0, false
		}
	}
	return rem, true
}

var recursiveMod10 = [10]int{0, 9, 4, 6, 8, 2, 7, 1, 3, 5}

func validCHQRReference(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 27 || strings.Trim(s, "0") == "" {
		return false
	}
	carry := 0
	for _, x := range d[:26] {
		carry = recursiveMod10[(carry+x)%10]
	}
	return (10-carry)%10 == d[26]
}
