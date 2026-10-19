package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// EU VAT numbers: a two-letter prefix and the member state's format. Where the member state publishes a
// check-digit rule (RO, BG, HU, DE, PL here) the number must pass it. Company VAT numbers are often public,
// so these start at low confidence and need a nearby keyword to be reported at the default level.
func init() {
	Register(Detector{
		Name:        "eu-vat",
		Label:       "EU VAT number",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\b(?:AT|BE|BG|CY|CZ|DE|DK|EE|EL|ES|FI|FR|HR|HU|IE|IT|LT|LU|LV|MT|NL|PL|PT|RO|SE|SI|SK|XI) ?[0-9A-Z+*]{2,13}`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validVAT,
		Needs:       Needs{Digits: 2, Check: hasVATPrefix},
		Keywords:    []string{"vat", "tva", "cui", "cif", "nip", "ust-id", "ustid", "iva", "btw", "moms", "dic", "dič", "ddv", "pdv", "ддс"},
		Concepts:    []Concept{VAT},
		Base:        domain.Low,
		Validation:  "matches the member state's VAT format, with its check digit where one is defined",
	})
}

// hasVATPrefix looks for a member-state prefix at the start of a word with a digit within the next three characters.
func hasVATPrefix(line string) bool {
	for i := 0; i+2 < len(line); i++ {
		if i > 0 && isWordByte(line[i-1]) || line[i] < 'A' || line[i] > 'Z' {
			continue
		}
		if _, ok := vatFormats[line[i:i+2]]; !ok {
			continue
		}
		for j := i + 2; j < len(line) && j < i+5; j++ {
			if line[j] >= '0' && line[j] <= '9' {
				return true
			}
		}
	}
	return false
}

func isWordByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}

var vatFormats = map[string]*regexp.Regexp{
	"AT": regexp.MustCompile(`^U\d{8}$`),
	"BE": regexp.MustCompile(`^[01]\d{9}$`),
	"BG": regexp.MustCompile(`^\d{9,10}$`),
	"CY": regexp.MustCompile(`^\d{8}[A-Z]$`),
	"CZ": regexp.MustCompile(`^\d{8,10}$`),
	"DE": regexp.MustCompile(`^\d{9}$`),
	"DK": regexp.MustCompile(`^\d{8}$`),
	"EE": regexp.MustCompile(`^10\d{7}$`),
	"EL": regexp.MustCompile(`^\d{9}$`),
	"ES": regexp.MustCompile(`^[A-Z0-9]\d{7}[A-Z0-9]$`),
	"FI": regexp.MustCompile(`^\d{8}$`),
	"FR": regexp.MustCompile(`^[A-HJ-NP-Z0-9]{2}\d{9}$`),
	"HR": regexp.MustCompile(`^\d{11}$`),
	"HU": regexp.MustCompile(`^\d{8}$`),
	"IE": regexp.MustCompile(`^(?:\d{7}[A-W][A-I]?|\d[A-Z+*]\d{5}[A-W])$`),
	"IT": regexp.MustCompile(`^\d{11}$`),
	"LT": regexp.MustCompile(`^(?:\d{9}|\d{12})$`),
	"LU": regexp.MustCompile(`^\d{8}$`),
	"LV": regexp.MustCompile(`^\d{11}$`),
	"MT": regexp.MustCompile(`^\d{8}$`),
	"NL": regexp.MustCompile(`^\d{9}B\d{2}$`),
	"PL": regexp.MustCompile(`^\d{10}$`),
	"PT": regexp.MustCompile(`^\d{9}$`),
	"RO": regexp.MustCompile(`^[1-9]\d{1,9}$`),
	"SE": regexp.MustCompile(`^\d{10}01$`),
	"SI": regexp.MustCompile(`^\d{8}$`),
	"SK": regexp.MustCompile(`^\d{10}$`),
	"XI": regexp.MustCompile(`^(?:\d{9}|\d{12}|GD\d{3}|HA\d{3})$`),
}

var vatChecks = map[string]func(string) bool{
	"RO": vatRO,
	"BG": vatBG,
	"HU": vatHU,
	"DE": func(s string) bool { return mod11_10(s) },
	"PL": vatPL,
}

func validVAT(s string) bool {
	if len(s) < 4 {
		return false
	}
	prefix, body := s[:2], s[2:]
	format, ok := vatFormats[prefix]
	if !ok || !format.MatchString(body) {
		return false
	}
	if check, ok := vatChecks[prefix]; ok {
		return check(body)
	}
	return true
}

// vatRO: the Romanian fiscal code (CIF). Pad to ten digits; weights 7,5,3,2,1,7,5,3,2 on the first nine;
// sum*10 mod 11, 10 becomes 0, equals the last digit.
func vatRO(s string) bool {
	d, ok := Digits(strings.Repeat("0", 10-len(s)) + s)
	if !ok {
		return false
	}
	w := []int{7, 5, 3, 2, 1, 7, 5, 3, 2}
	sum := 0
	for i, x := range w {
		sum += d[i] * x
	}
	c := sum * 10 % 11
	if c == 10 {
		c = 0
	}
	return c == d[9]
}

// vatBG: nine digits use weights 1..8, then 3..10 when the remainder is 10. Ten digits are personal numbers.
func vatBG(s string) bool {
	if len(s) == 10 {
		return validEGN(s) || validLNCh(s)
	}
	d, ok := Digits(s)
	if !ok {
		return false
	}
	sum := 0
	for i := 0; i < 8; i++ {
		sum += d[i] * (i + 1)
	}
	c := sum % 11
	if c == 10 {
		sum = 0
		for i := 0; i < 8; i++ {
			sum += d[i] * (i + 3)
		}
		c = sum % 11 % 10
	}
	return c == d[8]
}

// vatHU: weights 9,7,3,1,9,7,3 on the first seven digits; the eighth makes the sum a multiple of ten.
func vatHU(s string) bool {
	d, ok := Digits(s)
	if !ok {
		return false
	}
	w := []int{9, 7, 3, 1, 9, 7, 3}
	sum := 0
	for i, x := range w {
		sum += d[i] * x
	}
	return (10-sum%10)%10 == d[7]
}

// vatPL (NIP): weights 6,5,7,2,3,4,5,6,7, mod 11; a remainder of 10 is never issued.
func vatPL(s string) bool {
	d, ok := Digits(s)
	if !ok {
		return false
	}
	w := []int{6, 5, 7, 2, 3, 4, 5, 6, 7}
	sum := 0
	for i, x := range w {
		sum += d[i] * x
	}
	c := sum % 11
	return c != 10 && c == d[9]
}

// mod11_10 is ISO 7064 MOD 11,10, used by German VAT numbers and Croatian OIB.
func mod11_10(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) < 2 {
		return false
	}
	product := 10
	for _, x := range d[:len(d)-1] {
		sum := (x + product) % 10
		if sum == 0 {
			sum = 10
		}
		product = sum * 2 % 11
	}
	c := 11 - product
	if c == 10 {
		c = 0
	}
	return c == d[len(d)-1]
}
