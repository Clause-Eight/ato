package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// German health insurance number (Krankenversichertennummer, KVNR): the lifelong part printed on the health card
// is a capital letter, eight digits and a check digit. The letter becomes its two-digit place in the alphabet (A =
// 01, Z = 26); the ten digits before the check digit are weighted 1, 2, 1, 2…, two-digit products are replaced by
// the sum of their digits, and the check digit is the last digit of the total. It identifies a person's health
// insurance for life, so it is health-related data under GDPR Art. 9 in most uses.
// Sources: § 290 SGB V, https://www.gesetze-im-internet.de/sgb_5/__290.html (the number and its structure) and
// https://de.wikipedia.org/wiki/Krankenversichertennummer (the check digit, with a worked example).
func init() {
	Register(Detector{
		Name:        "de-kvnr",
		Label:       "German health insurance number (Krankenversichertennummer, KVNR)",
		Country:     "DE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[A-Z]\d{9}`),
		Validate:    validKVNR,
		Needs:       Needs{Check: func(line string) bool { return capsThenDigit(line, 1, 0) }},
		Keywords:    []string{"krankenversichertennummer", "versichertennummer", "versicherten-nr", "versichertennr", "kvnr", "kv-nr", "krankenversicherungsnummer"},
		Concepts:    []Concept{HealthInsurance},
		Base:        domain.Low,
		Validation:  "letter and Luhn-style check digit are valid",
	})
}

func validKVNR(s string) bool {
	if len(s) != 10 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	d, ok := Digits(s[1:])
	if !ok {
		return false
	}
	letter := int(s[0]-'A') + 1
	digits := append([]int{letter / 10, letter % 10}, d[:8]...)
	sum := 0
	for i, x := range digits {
		p := x * (1 + i%2)
		sum += p/10 + p%10
	}
	return sum%10 == d[8]
}
