package detectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// French social security number (NIR, numéro d'inscription au répertoire): S YY MM LLLLL OOO KK.
// S is the sex (1, 2; 7 and 8 for temporary numbers), MM the month (01-12, or 00, 20-42, 50-99 when the civil
// status is incomplete). LLLLL is the place of birth: a department (01-96, 2A, 2B) and a commune (001-999), or
// 97/98 with one digit for the overseas collectivity and a two-digit commune, or 99 with a continent digit (1-5)
// and a two-digit country. OOO is the order number (001-999). The key KK is 97 minus the first 13 digits mod 97,
// with 2A read as 19 and 2B as 18. Usually printed in groups: S YY MM DD CCC OOO KK.
// Source: DGFiP, Nomenclature des codes, annexe 2 "NIR" (January 2023), and the key rule as described by IREM
// Limoges, "Codes au quotidien" (https://www.irem.unilim.fr/OLD/fileadmin/documents/ERR/arithmetiquueaulycee/codes_au_quotidien.pdf).
//
// French tax number (numéro fiscal de référence, SPI): 13 digits, the first 0-3. The last three digits are the
// first ten read as a number, mod 511, padded with zeros. Usually printed in groups of 2, 2, 3, 3 and 3 digits.
// Source: DGFiP, "Règles de vérification de l'algorithme du numéro SPI"
// (https://www.impots.gouv.fr/sites/default/files/media/1_metier/3_partenaire/tiers_declarants/spi/algorithme_spi.pdf).
func init() {
	Register(Detector{
		Name:        "fr-nir",
		Label:       "French social security number (NIR)",
		Country:     "FR",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1278] ?\d{2} ?\d{2} ?(?:\d{2}|2[AB]) ?\d{3} ?\d{3} ?\d{2}`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validNIR,
		Formatted:   regexp.MustCompile(`^\d \d{2} \d{2} (?:\d{2}|2[AB]) \d{3} \d{3} \d{2}$`),
		Keywords:    []string{"nir", "numéro de sécurité sociale", "n° de sécurité sociale", "numéro sécu", "num secu", "numéro insee", "carte vitale"},
		Concepts:    []Concept{SocialSecurity, HealthInsurance},
		Base:        domain.Low,
		Validation:  "sex, month, place of birth and mod-97 check key are valid",
	})
	Register(Detector{
		Name:        "fr-spi",
		Label:       "French tax number (numéro fiscal, SPI)",
		Country:     "FR",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[0-3]\d ?\d{2} ?\d{3} ?\d{3} ?\d{3}`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validSPI,
		Formatted:   regexp.MustCompile(`^\d{2} \d{2} \d{3} \d{3} \d{3}$`),
		Keywords:    []string{"spi", "numéro fiscal", "numero fiscal de reference", "n° fiscal", "nif", "numéro d'identification fiscale"},
		Concepts:    []Concept{TaxID},
		Samples:     map[string]bool{"3023217600053": true},
		Base:        domain.Low,
		Validation:  "first digit and mod-511 check key are valid",
	})
}

func validSPI(s string) bool {
	if len(s) != 13 || s[0] < '0' || s[0] > '3' {
		return false
	}
	body, err := strconv.ParseInt(s[:10], 10, 64)
	if err != nil {
		return false
	}
	key, err := strconv.Atoi(s[10:])
	return err == nil && DigitsOnly(s) == s && body%511 == int64(key)
}

func validNIR(s string) bool {
	if len(s) != 15 {
		return false
	}
	body := s[:13]
	switch s[5:7] {
	case "2A":
		body = s[:5] + "19" + s[7:13]
	case "2B":
		body = s[:5] + "18" + s[7:13]
	}
	d, ok := Digits(body + s[13:])
	if !ok {
		return false
	}
	if d[0] != 1 && d[0] != 2 && d[0] != 7 && d[0] != 8 {
		return false
	}
	month := d[3]*10 + d[4]
	if month > 12 && month < 20 || month > 42 && month < 50 {
		return false
	}
	if !validNIRPlace(s[5:10]) || s[10:13] == "000" {
		return false
	}
	n, err := strconv.ParseInt(body, 10, 64)
	if err != nil {
		return false
	}
	return int(97-n%97) == d[13]*10+d[14]
}

// validNIRPlace checks the five characters of the place of birth.
func validNIRPlace(p string) bool {
	if p[:2] == "2A" || p[:2] == "2B" {
		return p[2:] != "000"
	}
	dept, err := strconv.Atoi(p[:2])
	if err != nil {
		return false
	}
	unit, rest := p[2], p[3:]
	switch dept {
	case 0:
		return false
	case 97:
		return unit >= '1' && unit <= '8' && rest != "00"
	case 98:
		return strings.IndexByte("4678", unit) >= 0 && rest != "00"
	case 99:
		return unit >= '1' && unit <= '5' && rest != "00"
	}
	return p[2:] != "000"
}
