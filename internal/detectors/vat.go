package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// EU VAT numbers: a two-letter prefix and the member state's format, and the number must pass that member state's
// check digit or check letter. Where a VAT number is a personal number (BG, CZ, ES, LV, SK) the personal number's
// own check applies. The rules follow the member states' published algorithms as collected by python-stdnum
// (https://arthurdejong.org/python-stdnum/). Company VAT numbers are often public,
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
		Validation:  "matches the member state's VAT format and its check digit or check letter",
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
	"AT": vatAT,
	"BE": vatBE,
	"BG": vatBG,
	"CY": vatCY,
	"CZ": vatCZ,
	"DE": func(s string) bool { return s[0] != '0' && mod11_10(s) },
	"DK": func(s string) bool { return s[0] != '0' && weightedMod(s, []int{2, 7, 6, 5, 4, 3, 2, 1}, 11) == 0 },
	"EE": func(s string) bool { return weightedMod(s, []int{3, 7, 1, 3, 7, 1, 3, 7, 1}, 10) == 0 },
	"EL": validGRAFM,
	"ES": vatES,
	"FI": func(s string) bool { return weightedMod(s, []int{7, 9, 10, 5, 8, 4, 2, 1}, 11) == 0 },
	"FR": vatFR,
	"HR": func(s string) bool { return mod11_10(s) },
	"HU": vatHU,
	"IE": vatIE,
	"IT": vatIT,
	"LT": vatLT,
	"LU": func(s string) bool { return atoi(s[:6])%89 == atoi(s[6:]) },
	"LV": vatLV,
	"MT": func(s string) bool { return s[0] != '0' && weightedMod(s, []int{3, 4, 6, 7, 8, 9, 10, 1}, 37) == 0 },
	"NL": vatNL,
	"PL": vatPL,
	"PT": vatPT,
	"RO": vatRO,
	"SE": func(s string) bool { return Luhn(s[:10]) },
	"SI": validSITaxNumber,
	"SK": vatSK,
	"XI": vatXI,
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

// weightedMod multiplies the leading digits of s by the weights and returns the sum mod m. s must be digits.
func weightedMod(s string, weights []int, m int) int {
	sum := 0
	for i, w := range weights {
		sum += int(s[i]-'0') * w
	}
	return sum % m
}

// atoi reads a run of digits that the format has already checked.
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// luhnSum is the Luhn checksum of a digit string: from the right, every second digit is doubled and its digits
// added. A number is Luhn-valid when the sum is 0.
func luhnSum(s string) int {
	sum := 0
	for i := len(s) - 1; i >= 0; i-- {
		x := int(s[i] - '0')
		if (len(s)-1-i)%2 == 1 {
			x *= 2
			x = x/10 + x%10
		}
		sum += x
	}
	return sum % 10
}

// vatAT: U and eight digits; the last is (6 - Luhn sum of digits 1-7) mod 10.
func vatAT(s string) bool { return (16-luhnSum(s[1:8]))%10 == int(s[8]-'0') }

// vatBE: the first eight digits plus the last two are a multiple of 97.
func vatBE(s string) bool { return atoi(s) > 0 && (atoi(s[:8])+atoi(s[8:]))%97 == 0 }

// vatCY: digits in even positions are translated (0→1, 1→0, 2→5, 3→7, 4→9, then 13, 15, 17, 19, 21), odd positions
// count as themselves; the sum mod 26 is the check letter. Numbers starting 12 are not issued.
func vatCY(s string) bool {
	if s[:2] == "12" {
		return false
	}
	tr := [10]int{1, 0, 5, 7, 9, 13, 15, 17, 19, 21}
	sum := 0
	for i := 0; i < 8; i++ {
		if i%2 == 0 {
			sum += tr[s[i]-'0']
		} else {
			sum += int(s[i] - '0')
		}
	}
	return s[8] == byte('A'+sum%26)
}

// vatCZ (DIČ): eight digits for companies, weights 8-2 and (11 - sum) mod 11, 0 read as 1, last digit of that;
// nine digits starting 6 for special cases; nine or ten digits otherwise are a birth number.
func vatCZ(s string) bool {
	switch {
	case len(s) == 8:
		if s[0] == '9' {
			return false
		}
		c := (11 - weightedMod(s, []int{8, 7, 6, 5, 4, 3, 2}, 11)) % 11
		if c == 0 {
			c = 1
		}
		return c%10 == int(s[7]-'0')
	case len(s) == 9 && s[0] == '6':
		c := weightedMod(s[1:], []int{8, 7, 6, 5, 4, 3, 2}, 11)
		return ((8-(10-c)%11)%10+10)%10 == int(s[8]-'0')
	}
	return validBirthNumber(s)
}

// vatES: a personal number (DNI, NIE, or K, L, M with the DNI letter) or a company CIF, whose control is the Luhn
// check digit of digits 2-8 or the letter JABCDEFGHI at that index.
func vatES(s string) bool {
	switch c := s[0]; {
	case c >= '0' && c <= '9':
		return validDNI(s)
	case strings.IndexByte("XYZ", c) >= 0:
		return validNIE(s)
	case strings.IndexByte("KLM", c) >= 0:
		return DigitsOnly(s[1:8]) == s[1:8] && dniLetters[atoi(s[1:8])%23] == s[8]
	case strings.IndexByte("ABCDEFGHJNPQRSUVW", c) >= 0:
		if DigitsOnly(s[1:8]) != s[1:8] {
			return false
		}
		check := (10 - luhnSum(s[1:8]+"0")) % 10
		return s[8] == byte('0'+check) || s[8] == "JABCDEFGHI"[check]
	}
	return false
}

// vatFR: a two-character key and the nine-digit SIREN, which passes Luhn unless it starts 000. A numeric key is
// (SIREN followed by 12) mod 97; a key with letters follows the newer mod-11 scheme.
func vatFR(s string) bool {
	const alphabet = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	siren := s[2:]
	if siren[:3] != "000" && luhnSum(siren) != 0 {
		return false
	}
	if DigitsOnly(s[:2]) == s[:2] {
		return atoi(s[:2]) == (atoi(siren)*100+12)%97
	}
	a, b := strings.IndexByte(alphabet, s[0]), strings.IndexByte(alphabet, s[1])
	check := a*34 + b - 100
	if a < 10 {
		check = a*24 + b - 10
	}
	return check >= 0 && (atoi(siren)+1+check/11)%11 == check%11
}

// vatIE: seven digits and a check letter (an optional second letter counts 9 times its index); old numbers with a
// letter or + or * second are rearranged first. The letter is WABCDEFGHIJKLMNOPQRSTUV at the weighted sum mod 23.
func vatIE(s string) bool {
	const alphabet = "WABCDEFGHIJKLMNOPQRSTUV"
	body, extra := s[:7], ""
	if len(s) == 9 {
		extra = s[8:]
	}
	if DigitsOnly(body) != body {
		body = "0" + s[2:7] + s[:1]
	}
	sum := 0
	for i := 0; i < 7; i++ {
		sum += (8 - i) * int(body[i]-'0')
	}
	if extra != "" {
		sum += 9 * strings.IndexByte(alphabet, extra[0])
	}
	return s[7] == alphabet[sum%23]
}

// vatIT (partita IVA): the first seven digits are not all zero, digits 8-10 are a province office code (001-100,
// 120, 121, 888, 999) and the whole number passes Luhn.
func vatIT(s string) bool {
	office := atoi(s[7:10])
	if atoi(s[:7]) == 0 || !(office >= 1 && office <= 100 || office == 120 || office == 121 || office == 888 || office == 999) {
		return false
	}
	return luhnSum(s) == 0
}

// vatLT: nine digits with 1 in position 8, or twelve with 1 in position 11. The check digit is the sum with
// weights 1-9 repeating, mod 11; if that is 10 the weights shift by two; 10 again becomes 0.
func vatLT(s string) bool {
	if len(s) == 9 && s[7] != '1' || len(s) == 12 && s[10] != '1' {
		return false
	}
	body := s[:len(s)-1]
	sum := 0
	for i := range body {
		sum += (1 + i%9) * int(body[i]-'0')
	}
	c := sum % 11
	if c == 10 {
		sum = 0
		for i := range body {
			sum += (1 + (i+2)%9) * int(body[i]-'0')
		}
		c = sum % 11
	}
	return c%10 == int(s[len(s)-1]-'0')
}

// vatLV: a first digit above 3 is a company, weights 9,1,4,8,3,10,2,5,7,6,1 giving 3 mod 11; otherwise it is a
// personal code.
func vatLV(s string) bool {
	if s[0] > '3' {
		return weightedMod(s, []int{9, 1, 4, 8, 3, 10, 2, 5, 7, 6, 1}, 11) == 3
	}
	return validLVPersonalCode(s)
}

// vatNL: nine digits, B and a two-digit suffix above 00. The digits pass the BSN eleven-test, or, for numbers
// issued to sole traders since 2020, NL plus the whole number passes ISO 7064 mod 97-10.
func vatNL(s string) bool {
	if atoi(s[:9]) == 0 || atoi(s[10:]) == 0 {
		return false
	}
	if validBSN(s[:9]) {
		return true
	}
	rem := 0
	for _, c := range "NL" + s {
		v := int(c - '0')
		if c >= 'A' && c <= 'Z' {
			v = int(c-'A') + 10
			rem = rem * 10 % 97
		}
		rem = (rem*10 + v) % 97
	}
	return rem == 1
}

// vatPT (NIF): no leading 0; weights 9-2 on the first eight digits; 11 minus the sum mod 11, 10 or 11 read as 0.
func vatPT(s string) bool {
	if s[0] == '0' {
		return false
	}
	c := 11 - weightedMod(s, []int{9, 8, 7, 6, 5, 4, 3, 2}, 11)
	if c >= 10 {
		c = 0
	}
	return c == int(s[8]-'0')
}

// vatSK (IČ DPH): a birth number, or a company number not starting 0, with 2, 3, 4, 7, 8 or 9 third, divisible
// by 11.
func vatSK(s string) bool {
	if validBirthNumber(s) {
		return true
	}
	return s[0] != '0' && strings.IndexByte("234789", s[2]) >= 0 && atoi(s)%11 == 0
}

// vatXI: the UK scheme for Northern Ireland. Nine digits (twelve with a branch suffix) weighted 8,7,6,5,4,3,2,10,1
// mod 97 give 0, or 42 or 55 for numbers from 100 upwards; GD is below 500, HA from 500.
func vatXI(s string) bool {
	switch {
	case strings.HasPrefix(s, "GD"):
		return atoi(s[2:]) < 500
	case strings.HasPrefix(s, "HA"):
		return atoi(s[2:]) >= 500
	}
	c := weightedMod(s, []int{8, 7, 6, 5, 4, 3, 2, 10, 1}, 97)
	return c == 0 || atoi(s[:3]) >= 100 && (c == 42 || c == 55)
}
