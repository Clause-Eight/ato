package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Greek social security number (AMKA, Αριθμός Μητρώου Κοινωνικής Ασφάλισης): eleven digits, DDMMYY birth
// date, a four-digit serial (odd for men, even for women) and a Luhn check digit. Introduced by article 64 of
// law 2084/1992 and made the single insurance identifier by law 3655/2008. Source:
// https://el.wikipedia.org/wiki/Αριθμός_Μητρώου_Κοινωνικής_Ασφάλισης (structure and check, citing amka.gr).
//
// Greek tax number (AFM, Αριθμός Φορολογικού Μητρώου): nine digits; the first eight are weighted 256, 128 …
// 2 (2^8 down to 2^1), and the sum mod 11 mod 10 is the ninth digit. The Greek VAT number is the same number
// with the EL prefix, which eu-vat finds. Sources: the nine-digit format (article 11 of law 4174/2013),
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Greece-TIN.pdf
// and the check rule, https://lookuptax.com/docs/tax-identification-number/greece-tax-id-guide (no AADE text
// for it was found).
func init() {
	Register(Detector{
		Name:        "gr-amka",
		Label:       "Greek social security number (AMKA)",
		Country:     "GR",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{11}`),
		Validate:    validGRAMKA,
		Keywords:    []string{"αμκα", "α.μ.κ.α.", "amka"},
		Concepts:    []Concept{SocialSecurity},
		Base:        domain.Low,
		Validation:  "birth date and Luhn check digit are valid",
	})
	Register(Detector{
		Name:        "gr-afm",
		Label:       "Greek tax number (AFM)",
		Country:     "GR",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{9}`),
		Validate:    validGRAFM,
		Keywords:    []string{"αφμ", "α.φ.μ.", "afm", "αριθμός φορολογικού μητρώου"},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Validation:  "mod-11 check digit is valid",
	})
}

func validGRAMKA(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 {
		return false
	}
	day, month, yy := d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]
	if !ValidDate(1900+yy, month, day) && !ValidDate(2000+yy, month, day) {
		return false
	}
	return Luhn(s)
}

func validGRAFM(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 {
		return false
	}
	sum, zero := 0, true
	for i := 0; i < 8; i++ {
		sum += d[i] << (8 - i)
		zero = zero && d[i] == 0
	}
	return !zero && sum%11%10 == d[8]
}
