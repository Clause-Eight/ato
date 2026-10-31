package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Polish identity card number (seria i numer dowodu osobistego): three letters for the series and six digits,
// the first of which is the check digit. Letters count A=10 … Z=35; the nine values are weighted
// 7,3,1,9,7,3,1,7,3 and the sum must be divisible by 10. Series AAA is not issued.
// Source: https://pl.wikipedia.org/wiki/Dowód_osobisty_w_Polsce (section on the number's check digit).
//
// Polish business register number (REGON): nine digits for an entity, fourteen for a local unit. The ninth digit
// is the sum of the first eight weighted 8,9,2,3,4,5,6,7, mod 11 (10 becomes 0); a fourteen-digit number also
// starts with a valid nine-digit REGON and ends with the sum of the first thirteen weighted
// 2,4,8,5,0,9,7,3,6,1,2,4,8, mod 11 (10 becomes 0). It names a person only for sole traders, so it needs a keyword.
// Source: https://pl.wikipedia.org/wiki/REGON, describing the rules of the GUS business register.
//
// Polish tax identification number (NIP, numer identyfikacji podatkowej): ten digits, the first three naming the
// tax office that issued it. The first nine are weighted 6,5,7,2,3,4,5,6,7; the sum mod 11 is the tenth digit, and
// a remainder of 10 is never issued. It is usually printed 999-999-99-99 or 999-99-99-999. Since September 2011
// people who run no business and are not VAT registered use their PESEL instead, so a NIP names a sole trader or a
// company; the PL-prefixed VAT form is left to eu-vat. Source: the Polish TIN sheet published by the OECD, section
// 2.2, with its worked example,
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Poland-TIN.pdf
func init() {
	Register(Detector{
		Name:         "pl-id-card",
		Label:        "Polish identity card number (dowód osobisty)",
		Country:      "PL",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[A-Z]{3} ?\d{6}`),
		Normalize:    func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:     validPLIDCard,
		Needs:        Needs{Check: func(line string) bool { return capsThenDigit(line, 3, 1) }},
		Keywords:     []string{"dowód osobisty", "dowod osobisty", "numer dowodu", "nr dowodu", "seria dowodu", "dowód", "dowod"},
		Concepts:     []Concept{IDCard},
		Base:         domain.Low,
		Validation:   "check digit over the series letters and number is valid",
		NeedsKeyword: true,
	})
	Register(Detector{
		Name:         "pl-regon",
		Label:        "Polish business register number (REGON)",
		Country:      "PL",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`\d{9}(?:\d{5})?`),
		Validate:     validREGON,
		Keywords:     []string{"regon", "numer regon", "nr regon"},
		Base:         domain.Low,
		NeedsKeyword: true,
		Validation:   "mod-11 check digits are valid",
	})
	Register(Detector{
		Name:        "pl-nip",
		Label:       "Polish tax identification number (NIP)",
		Country:     "PL",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{3}-?\d{3}-?\d{2}-?\d{2}|\d{3}-\d{2}-\d{2}-\d{3}`),
		Normalize:   DigitsOnly,
		Validate:    validPLNIP,
		Formatted:   regexp.MustCompile(`^(?:\d{3}-\d{3}-\d{2}-\d{2}|\d{3}-\d{2}-\d{2}-\d{3})$`),
		Keywords:    []string{"nip", "numer nip", "nr nip", "numer identyfikacji podatkowej"},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Validation:  "mod-11 check digit is valid",
	})
}

func validPLNIP(s string) bool {
	return len(s) == 10 && s != "0000000000" && vatPL(s)
}

var plIDCardWeights = [9]int{7, 3, 1, 9, 7, 3, 1, 7, 3}

func validPLIDCard(s string) bool {
	if len(s) != 9 || s[:3] == "AAA" {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		c := s[i]
		var v int
		switch {
		case i < 3 && c >= 'A' && c <= 'Z':
			v = int(c-'A') + 10
		case i >= 3 && c >= '0' && c <= '9':
			v = int(c - '0')
		default:
			return false
		}
		sum += v * plIDCardWeights[i]
	}
	return sum%10 == 0
}

var (
	regon9Weights  = []int{8, 9, 2, 3, 4, 5, 6, 7}
	regon14Weights = []int{2, 4, 8, 5, 0, 9, 7, 3, 6, 1, 2, 4, 8}
)

func validREGON(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 && len(d) != 14 {
		return false
	}
	if !regonCheck(d[:9], regon9Weights) {
		return false
	}
	return len(d) == 9 || regonCheck(d, regon14Weights)
}

// regonCheck reports whether the digit after the weighted ones is their sum mod 11, with 10 written as 0.
func regonCheck(d []int, weights []int) bool {
	sum := 0
	for i, w := range weights {
		sum += d[i] * w
	}
	return sum%11%10 == d[len(weights)]
}
