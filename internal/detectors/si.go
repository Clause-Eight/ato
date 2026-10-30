package detectors

import (
	"regexp"
	"time"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Slovenian personal number (EMŠO, enotna matična številka občana): thirteen digits DDMMYYY RR SSS K. The
// birth date has a three-digit year (9xx is 19xx, 0xx is 20xx); RR is the register, 50 for Slovenia, widened
// to 50-59 by the 2024 amendment of the Central Population Register Act (ZCRP); SSS is 000-499 for men and
// 500-999 for women. Digits 1-12 are weighted 7,6,5,4,3,2,7,6,5,4,3,2; a remainder of 0 gives check 0, a
// remainder of 1 is never issued, otherwise the check is 11 minus the remainder. Sources: Uredba o načinu
// določanja osebne identifikacijske številke, Uradni list RS 8/1999,
// https://www.uradni-list.si/glasilo-uradni-list-rs/vsebina/1999-01-0345 and article 8 of ZCRP as amended by
// ZCRP-B, Uradni list RS 32/2024 (registers 50-59).
// EMŠOs from the other former Yugoslav registers are not accepted: they belong to other countries' numbering.
//
// Slovenian tax number (davčna številka): eight digits. The first seven are a random number from 1000000 to
// 9999999, so the first digit is never 0; the eighth is a mod-11 check digit. The first seven digits are weighted
// 8,7,6,5,4,3,2 from the left; the check digit is 11 minus the sum mod 11, written 0 when that is 10. A remainder
// of 0 would need a check of 11, so those numbers are not issued. The same number with the SI prefix is the VAT
// number, which eu-vat finds. Sources: the Slovenian TIN sheet published by the OECD (format, first digit, mod 11),
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Slovenia-TIN.pdf
// and the European Commission's TIN-on-the-Web syntax check, checkTinService (weights and the remainder rules),
// https://taxation-customs.ec.europa.eu/online-services/online-services-and-databases-taxation/taxpayer-identification-number-tin_en
func init() {
	Register(Detector{
		Name:        "si-emso",
		Label:       "Slovenian personal number (EMŠO)",
		Country:     "SI",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{13}`),
		Validate:    validSIEMSO,
		Keywords:    []string{"emšo", "emso", "enotna matična številka", "enotna maticna stevilka"},
		Concepts:    []Concept{NationalID},
		Unlikely:    referenceWords(),
		Base:        domain.Medium,
		Samples:     map[string]bool{"2902932505526": true},
		Validation:  "birth date, register and mod-11 check digit are valid",
	})
	Register(Detector{
		Name:        "si-tax-number",
		Label:       "Slovenian tax number (davčna številka)",
		Country:     "SI",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[1-9]\d{7}`),
		Validate:    validSITaxNumber,
		Keywords:    []string{"davčna številka", "davcna stevilka", "davčna št.", "davcna st."},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Validation:  "mod-11 check digit is valid",
	})
}

func validSITaxNumber(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 8 || d[0] == 0 {
		return false
	}
	sum := 0
	for i := 0; i < 7; i++ {
		sum += d[i] * (8 - i)
	}
	rem := sum % 11
	if rem == 0 {
		return false
	}
	return (11-rem)%10 == d[7]
}

var siEMSOWeights = [12]int{7, 6, 5, 4, 3, 2, 7, 6, 5, 4, 3, 2}

func validSIEMSO(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 13 {
		return false
	}
	day, month, yyy := d[0]*10+d[1], d[2]*10+d[3], d[4]*100+d[5]*10+d[6]
	var year int
	switch d[4] {
	case 9:
		year = 1000 + yyy
	case 0:
		year = 2000 + yyy
	default:
		return false
	}
	if year > time.Now().Year() || !ValidDate(year, month, day) {
		return false
	}
	if d[7] != 5 {
		return false
	}
	sum := 0
	for i, w := range siEMSOWeights {
		sum += d[i] * w
	}
	rem := sum % 11
	if rem == 1 {
		return false
	}
	return (11-rem)%11 == d[12]
}
