package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// IBAN (ISO 13616): country code, two check digits, a national account number of fixed length per country.
// Valid when the length matches the country and the number mod 97 is 1.
func init() {
	Register(Detector{
		Name:        "iban",
		Label:       "Bank account number (IBAN)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?i)[a-z]{2}\d{2}(?:[ -]?[a-z0-9]{4}){2,7}(?:[ -]?[a-z0-9]{1,3})?`),
		Normalize:   normalizeIBAN,
		Validate:    validIBAN,
		Mask:        maskIBAN,
		Needs:       Needs{Digits: 10},
		Keywords:    []string{"iban", "account", "cont bancar", "bankszámla", "bankszamla", "сметка"},
		Concepts:    []Concept{BankAccount},
		Base:        domain.High,
		Samples: map[string]bool{
			"DE89370400440532013000":      true,
			"GB82WEST12345698765432":      true,
			"GB29NWBK60161331926819":      true,
			"FR1420041010050500013M02606": true,
			"RO49AAAA1B31007593840000":    true,
			"NL91ABNA0417164300":          true,
			"BE68539007547034":            true,
		},
		TrimGroups: true,
		Validation: "country length and mod-97 check are valid",
	})
}

// IBANLengths is the IBAN length for each country in the SEPA area, plus a few common neighbours.
var IBANLengths = map[string]int{
	"AD": 24, "AT": 20, "BE": 16, "BG": 22, "CH": 21, "CY": 28, "CZ": 24, "DE": 22, "DK": 18, "EE": 20,
	"ES": 24, "FI": 18, "FO": 18, "FR": 27, "GB": 22, "GI": 23, "GL": 18, "GR": 27, "HR": 21, "HU": 28,
	"IE": 22, "IS": 26, "IT": 27, "LI": 21, "LT": 20, "LU": 20, "LV": 21, "MC": 27, "MD": 24, "ME": 22,
	"MK": 19, "MT": 31, "NL": 18, "NO": 15, "PL": 28, "PT": 25, "RO": 24, "RS": 22, "SE": 24, "SI": 19,
	"SK": 24, "SM": 27, "UA": 29, "VA": 22, "AL": 28, "BA": 20, "TR": 26,
}

func normalizeIBAN(s string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(s))
}

func validIBAN(s string) bool {
	if len(s) < 5 {
		return false
	}
	want, ok := IBANLengths[s[:2]]
	if !ok || len(s) != want {
		return false
	}
	rearranged := s[4:] + s[:4]
	rem := 0
	for i := 0; i < len(rearranged); i++ {
		c := rearranged[i]
		switch {
		case c >= '0' && c <= '9':
			rem = (rem*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			rem = (rem*100 + int(c-'A'+10)) % 97
		default:
			return false
		}
	}
	return rem == 1
}

func maskIBAN(s string) string {
	r := []rune(s)
	if len(r) <= 6 {
		return MaskAll(s)
	}
	return string(r[:2]) + strings.Repeat("•", len(r)-6) + string(r[len(r)-4:])
}
