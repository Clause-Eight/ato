package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// UK National Insurance number (NINO): two prefix letters, six digits and a suffix A, B, C or D, often printed
// "AB 12 34 56 C". The suffix is required here: without it the shape is shared by many other codes (Romanian ID
// cards, part numbers). D, F, I, Q, U and V are never prefix letters, O is never the second, and the prefixes BG,
// GB, KN, NK, NT, TN and ZZ are not allocated. There is no check digit, so it starts low and is reported at the
// default level with a keyword or in the printed format.
// Source: HMRC National Insurance Manual NIM39110, https://www.gov.uk/hmrc-internal-manuals/national-insurance-manual/nim39110
//
// UK NHS number: ten digits, printed 3-3-4. The check digit is 11 minus the first nine digits weighted 10 down to
// 2, mod 11; 11 is written 0 and 10 is never issued. Numbers are issued in 400-499 and 600-799 million (England,
// Wales, Isle of Man) and 320-399 million (Northern Ireland); Scotland's CHI numbers use the same check and start
// with the date of birth (DDMMYY). The 999 range is reserved for tests and is rejected.
// Source: https://en.wikipedia.org/wiki/NHS_number, describing the NHS Data Dictionary rule.
func init() {
	Register(Detector{
		Name:        "gb-nino",
		Label:       "UK National Insurance number (NINO)",
		Country:     "GB",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`[A-CEGHJ-PR-TW-Z][A-CEGHJ-NPR-TW-Z] ?\d{2} ?\d{2} ?\d{2} ?[A-D]`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validNINO,
		Formatted:   regexp.MustCompile(`^[A-Z]{2} \d{2} \d{2} \d{2} [A-D]$`),
		Needs:       Needs{Check: func(line string) bool { return capsThenDigit(line, 2, 1) }},
		Keywords:    []string{"nino", "national insurance", "national insurance number", "ni number", "ni no"},
		Concepts:    []Concept{SocialSecurity},
		Base:        domain.Low,
		Validation:  "prefix is allocated and the suffix is A to D",
	})
	Register(Detector{
		Name:        "gb-nhs",
		Label:       "UK NHS number",
		Country:     "GB",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`\d{3}[ -]?\d{3}[ -]?\d{4}`),
		Normalize:   DigitsOnly,
		Validate:    validNHS,
		Formatted:   regexp.MustCompile(`^\d{3}([ -])\d{3}([ -])\d{4}$`),
		Keywords:    []string{"nhs", "nhs number", "nhs no", "nhs_number", "nhsnumber", "chi number", "chi no"},
		Concepts:    []Concept{HealthInsurance},
		Base:        domain.Low,
		Validation:  "issued range and mod-11 check digit are valid",
	})
}

var ninoUnallocated = map[string]bool{"BG": true, "GB": true, "KN": true, "NK": true, "NT": true, "TN": true, "ZZ": true}

func validNINO(s string) bool {
	if len(s) != 9 {
		return false
	}
	p := s[:2]
	for i := 0; i < 2; i++ {
		if p[i] < 'A' || p[i] > 'Z' || strings.IndexByte("DFIQUV", p[i]) >= 0 {
			return false
		}
	}
	if p[1] == 'O' || ninoUnallocated[p] {
		return false
	}
	if _, ok := Digits(s[2:8]); !ok {
		return false
	}
	return s[8] >= 'A' && s[8] <= 'D'
}

func validNHS(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 {
		return false
	}
	million := d[0]*100 + d[1]*10 + d[2]
	switch {
	case million <= 311:
		if !ValidDate(2000, d[2]*10+d[3], d[0]*10+d[1]) {
			return false
		}
	case million >= 320 && million <= 499, million >= 600 && million <= 799:
	default:
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += d[i] * (10 - i)
	}
	c := (11 - sum%11) % 11
	return c != 10 && c == d[9]
}
