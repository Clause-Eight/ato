package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Luxembourg national identification number (matricule, numéro d'identification national): thirteen digits
// YYYY MM DD SSS C1 C2, the birth date, a three-digit serial and two check digits. C1 is the Luhn check digit of the
// first eleven digits and C2 the Verhoeff check digit of the same eleven digits (not of the twelve). Sources: the
// Luxembourg TIN sheet published by the OECD,
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Luxembourg-TIN.pdf
// and the tax administration's page, https://impotsdirects.public.lu/fr/salpens/fiche/date_naiss.html. Verhoeff's
// check uses the dihedral group of order 10, https://en.wikipedia.org/wiki/Verhoeff_algorithm.
func init() {
	Register(Detector{
		Name:        "lu-national-id",
		Label:       "Luxembourg national identification number (matricule)",
		Country:     "LU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:19|20)\d{2} ?\d{4} ?\d{3} ?\d{2}`),
		Normalize:   DigitsOnly,
		Validate:    validLUNationalID,
		Formatted:   regexp.MustCompile(`^\d{4} \d{4} \d{3} \d{2}$`),
		Keywords: []string{"matricule", "numéro matricule", "numéro de matricule", "matricule national",
			"numéro d'identification national", "nationale identifikationsnummer"},
		Concepts:   []Concept{NationalID, SocialSecurity, TaxID},
		Base:       domain.Medium,
		Validation: "birth date, Luhn and Verhoeff check digits are valid",
	})
}

func validLUNationalID(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 13 {
		return false
	}
	year := d[0]*1000 + d[1]*100 + d[2]*10 + d[3]
	if year < 1900 || year > 2099 || !ValidBirthDate(year, d[4]*10+d[5], d[6]*10+d[7]) {
		return false
	}
	return Luhn(s[:12]) && verhoeffDigit(s[:11]) == d[12]
}

// d5 multiplies two elements of the dihedral group of order 10, numbered as Verhoeff does: 0-4 are the rotations
// r^k and 5-9 the reflections r^k s. Since s r = r^-1 s, (r^a s^f)(r^b s^g) = r^(a ± b) s^(f xor g).
func d5(x, y int) int {
	a, f := x%5, x/5
	b, g := y%5, y/5
	if f == 1 {
		b = 5 - b
	}
	return (a+b)%5 + 5*(f^g)
}

// verhoeffPerm is the base permutation (1 5 8 9 4 2 7 0)(3 6); position i of a number uses it i times.
var verhoeffPerm = [10]int{1, 5, 7, 6, 2, 8, 3, 0, 9, 4}

// verhoeffDigit is the Verhoeff check digit of a digit string: the group inverse of the product of the digits,
// each permuted by its position counted from the right starting at 1.
func verhoeffDigit(body string) int {
	c := 0
	for i := len(body) - 1; i >= 0; i-- {
		n := int(body[i] - '0')
		for k := 0; k < (len(body)-i)%8; k++ {
			n = verhoeffPerm[n]
		}
		c = d5(c, n)
	}
	if c < 5 {
		return (5 - c) % 5
	}
	return c
}
