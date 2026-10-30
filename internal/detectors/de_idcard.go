package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// German ID card (Personalausweis) document number, issued since November 2010: nine characters from the digits
// and the consonants C F G H J K L M N P R T V W X Y Z, the first one of L M N P R T V W X Y (passports use
// C F G H J K), followed on the back and in the machine-readable zone by a check digit. The check digit follows
// ICAO Doc 9303 Part 3: digits keep their value, A-Z count 10-35, the weights 7, 3, 1 repeat, and the sum mod 10
// is the digit. Only the ten-character form with its check digit is matched.
// Sources: "Die alphanumerische Seriennummer im neuen Personalausweis" (Marktflecken Villmar, Ausweisrecht),
// https://www.marktflecken-villmar.de/fileadmin/villmar/downloads/ausweisrecht/Seriennummer-des-Personalausweises.pdf
// and ICAO Doc 9303 Part 3, section 4.9, https://www.icao.int/publications/pages/publication.aspx?docnum=9303
func init() {
	Register(Detector{
		Name:        "de-id-card",
		Label:       "German ID card number (Personalausweis)",
		Country:     "DE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[LMNPRTVWXY][0-9CFGHJKLMNPRTVWXYZ]{8}\d`),
		Validate:    validDEIDCard,
		Needs:       Needs{Check: hasDEIDCardStart},
		Keywords:    []string{"personalausweisnummer", "ausweisnr", "ausweis-nr", "perso-nr"},
		Concepts:    []Concept{IDCard},
		Base:        domain.Low,
		Validation:  "alphabet and ICAO 9303 check digit are valid",
	})
}

const (
	deIDCardFirst    = "LMNPRTVWXY"
	deIDCardAlphabet = "0123456789CFGHJKLMNPRTVWXYZ"
)

func validDEIDCard(s string) bool {
	if len(s) != 10 || strings.IndexByte(deIDCardFirst, s[0]) < 0 {
		return false
	}
	for i := 1; i < 9; i++ {
		if strings.IndexByte(deIDCardAlphabet, s[i]) < 0 {
			return false
		}
	}
	if s[9] < '0' || s[9] > '9' {
		return false
	}
	return icaoCheckDigit(s[:9]) == int(s[9]-'0')
}

// icaoCheckDigit is the ICAO 9303 check digit: digits as values, A-Z as 10-35, anything else ('<') as 0,
// weights 7, 3, 1.
func icaoCheckDigit(s string) int {
	weights := [3]int{7, 3, 1}
	sum := 0
	for i := 0; i < len(s); i++ {
		c, v := s[i], 0
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c >= 'A' && c <= 'Z':
			v = int(c-'A') + 10
		}
		sum += v * weights[i%3]
	}
	return sum % 10
}

// hasDEIDCardStart looks for a first letter of an ID card number at the start of a word, followed by two
// characters of the document alphabet.
func hasDEIDCardStart(line string) bool {
	for i := 0; i+2 < len(line); i++ {
		if i > 0 && isWordByte(line[i-1]) || strings.IndexByte(deIDCardFirst, line[i]) < 0 {
			continue
		}
		if strings.IndexByte(deIDCardAlphabet, line[i+1]) >= 0 && strings.IndexByte(deIDCardAlphabet, line[i+2]) >= 0 {
			return true
		}
	}
	return false
}
