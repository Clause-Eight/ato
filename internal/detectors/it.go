package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Italian tax code (codice fiscale): 16 characters. Three letters from the surname, three from the name, the
// year (2 digits), a month letter (A B C D E H L M P R S T), the day (01-31, plus 40 for women), the cadastral
// code of the municipality or Z and a foreign country code (a letter and 3 digits), and a check letter. The check
// letter is the sum of the first 15 characters' values mod 26, read as A-Z: odd positions (1, 3, ...) use the
// "dispari" table, even positions their plain value (0-9, A-Z as 0-25). For people with the same code (omocodia)
// any of the seven digits (positions 7, 8, 10, 11, 13, 14, 15) may be replaced by L M N P Q R S T U V for 0-9.
// Source: Agenzia delle Entrate rules as described by Fiscomania, "Carattere di controllo"
// (https://fiscomania.com/carattere-controllo) and "Omocodia" (https://fiscomania.com/omocodia).
func init() {
	Register(Detector{
		Name:        "it-fiscal-code",
		Label:       "Italian tax code (codice fiscale)",
		Country:     "IT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[A-Z]{6}[0-9LMNP-V]{2}[ABCDEHLMPRST][0-9LMNP-V]{2}[A-Z][0-9LMNP-V]{3}[A-Z]`),
		Validate:    validFiscalCode,
		Needs:       Needs{Check: hasUpperRun16},
		Keywords:    []string{"codice fiscale", "codicefiscale", "cod fisc", "cod. fisc.", "c.f.", "cf"},
		Concepts:    []Concept{TaxID, NationalID},
		Base:        domain.Medium,
		Validation:  "date, municipality code and check letter are valid",
	})
}

// fiscalOdd is the value of 0-9 and A-Z in an odd position.
var fiscalOdd = [26]int{1, 0, 5, 7, 9, 13, 15, 17, 19, 21, 2, 4, 18, 20, 11, 3, 6, 8, 12, 14, 16, 10, 22, 25, 24, 23}

const omocodeLetters = "LMNPQRSTUV"

func validFiscalCode(s string) bool {
	if len(s) != 16 {
		return false
	}
	plain := []byte(s)
	for _, i := range []int{6, 7, 9, 10, 12, 13, 14} {
		if j := strings.IndexByte(omocodeLetters, plain[i]); j >= 0 {
			plain[i] = byte('0' + j)
		}
	}
	d, ok := Digits(string(plain[6:8]) + string(plain[9:11]) + string(plain[12:15]))
	if !ok {
		return false
	}
	month := strings.IndexByte("ABCDEHLMPRST", s[8]) + 1
	day := d[2]*10 + d[3]
	if day > 40 {
		day -= 40
	}
	if month == 0 || !ValidDate(2000, month, day) {
		return false
	}
	if c := s[11]; !(c >= 'A' && c <= 'M' || c == 'Z') {
		return false
	}
	sum := 0
	for i := 0; i < 15; i++ {
		c := s[i]
		v := int(c - 'A')
		if c >= '0' && c <= '9' {
			v = int(c - '0')
		}
		if v < 0 || v > 25 {
			return false
		}
		if i%2 == 0 {
			v = fiscalOdd[v]
		}
		sum += v
	}
	return s[15] == byte('A'+sum%26)
}

// hasUpperRun16 reports whether the line holds 16 capital letters and digits in a row.
func hasUpperRun16(line string) bool {
	run := 0
	for i := 0; i < len(line); i++ {
		if c := line[i]; c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			if run++; run >= 16 {
				return true
			}
			continue
		}
		run = 0
	}
	return false
}
