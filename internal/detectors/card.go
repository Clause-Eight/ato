package detectors

import (
	"regexp"
	"strconv"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Payment card numbers: a known brand prefix, a length that brand issues, and the Luhn check.
// Output keeps only the last four digits.
func init() {
	Register(Detector{
		Name:        "card",
		Label:       "Payment card number",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{4}[ -]?\d{6}[ -]?\d{5}|\d{4}(?:[ -]?\d{4}){2}(?:[ -]?\d{1,4}){1,2}`),
		Normalize:   DigitsOnly,
		Validate:    func(s string) bool { return CardBrand(s) != "" },
		Mask:        MaskKeepLast(4),
		Keywords:    []string{"card", "pan", "cc_number", "ccnumber", "credit", "debit", "visa", "mastercard", "maestro", "amex", "card_number", "cardnumber"},
		Concepts:    []Concept{Card},
		Unlikely:    []string{"id", "trace", "order", "invoice", "transaction", "txn", "tracking", "correlation", "request", "session", "sequence", "nonce", "timestamp", "serial", "reference", "ref", "build"},
		Base:        domain.Medium,
		Samples: map[string]bool{
			"4111111111111111": true, "4242424242424242": true, "4012888888881881": true, "4000056655665556": true,
			"5555555555554444": true, "5105105105105100": true, "2223003122003222": true,
			"378282246310005": true, "371449635398431": true, "6011111111111117": true, "3530111333300000": true,
		},
		TrimGroups: true,
		Formatted:  regexp.MustCompile(`^\d{4} \d{4} \d{4} \d{4}$|^\d{4}-\d{4}-\d{4}-\d{4}$|^\d{4} \d{6} \d{5}$|^\d{4}-\d{6}-\d{5}$`),
		Validation: "brand prefix, length and Luhn check are valid",
	})
}

type cardRange struct {
	brand   string
	low, hi int
	digits  int
	lengths []int
}

var cardRanges = []cardRange{
	{"Visa", 4, 4, 1, []int{13, 16}},
	{"Mastercard", 51, 55, 2, []int{16}},
	{"Mastercard", 2221, 2720, 4, []int{16}},
	{"American Express", 34, 34, 2, []int{15}},
	{"American Express", 37, 37, 2, []int{15}},
	{"Discover", 6011, 6011, 4, []int{16, 19}},
	{"Discover", 644, 649, 3, []int{16, 19}},
	{"Discover", 65, 65, 2, []int{16, 19}},
	{"JCB", 3528, 3589, 4, []int{16, 17, 18, 19}},
	{"Maestro", 50, 50, 2, []int{12, 13, 14, 15, 16, 17, 18, 19}},
	{"Maestro", 56, 69, 2, []int{12, 13, 14, 15, 16, 17, 18, 19}},
}

// CardBrand returns the brand of a valid card number, or "" if the number is not a valid card.
func CardBrand(s string) string {
	if len(s) < 12 || len(s) > 19 || !Luhn(s) {
		return ""
	}
	for _, r := range cardRanges {
		prefix, err := strconv.Atoi(s[:r.digits])
		if err != nil || prefix < r.low || prefix > r.hi {
			continue
		}
		for _, l := range r.lengths {
			if l == len(s) {
				return r.brand
			}
		}
	}
	return ""
}

// Luhn reports whether a digit string passes the Luhn (mod 10) check.
func Luhn(s string) bool {
	sum, double := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		n := int(c - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}
