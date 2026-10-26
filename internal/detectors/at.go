package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Austrian social insurance number (Sozialversicherungsnummer, SVNR or VSNR): ten digits LLL P TT MM JJ. LLL is a
// running number from 100 upwards, P the check digit and TTMMJJ the birth date. P is the sum of the other nine
// digits weighted 3, 7, 9 (running number) and 5, 8, 4, 2, 1, 6 (date), taken mod 11; a remainder of 10 is never
// issued, the next running number is used instead. When the birth date is unknown, or all running numbers for a
// date are used up, the month is written as 13, 14 or 15. Source: the Federal Ministry's answer to parliamentary
// question 4690/AB XXIII. GP, https://www.parlament.gv.at/dokument/XXIII/AB/4690/fnameorig_136871.html
//
// Austrian tax number (Steuernummer, also the Abgabenkontonummer): nine digits, printed 99-999/9999. The first two
// digits name the tax office, the next six are the taxpayer's number at that office and the last is a check digit:
// the first eight digits are weighted 1, 2, 1, 2 … from the left, a product of two digits counts as the sum of its
// digits, and the check digit brings the total up to a multiple of ten (the Luhn rule over all nine digits). The
// hyphen and slash are often left out. Sources: the Austrian TIN sheet published by the OECD (format),
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Austria-TIN.pdf
// and the European Commission's TIN-on-the-Web syntax check, checkTinService, which accepts exactly these numbers
// (https://taxation-customs.ec.europa.eu/online-services/online-services-and-databases-taxation/taxpayer-identification-number-tin_en).
// The check accepts every office prefix, so none is enforced here.
func init() {
	Register(Detector{
		Name:        "at-social-insurance",
		Label:       "Austrian social insurance number (Sozialversicherungsnummer)",
		Country:     "AT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{4} ?\d{6}`),
		Normalize:   DigitsOnly,
		Validate:    validATSocialInsurance,
		Formatted:   regexp.MustCompile(`^\d{4} \d{6}$`),
		Keywords:    []string{"vsnr", "sv-nr", "sv-nummer", "svnummer", "e-card", "ecard"},
		Concepts:    []Concept{SocialSecurity, HealthInsurance},
		Base:        domain.Low,
		Validation:  "birth date and check digit are valid",
	})
	Register(Detector{
		Name:        "at-tax-number",
		Label:       "Austrian tax number (Steuernummer)",
		Country:     "AT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{2}[- ]?\d{3}/?\d{4}`),
		Normalize:   DigitsOnly,
		Validate:    validATTaxNumber,
		Formatted:   regexp.MustCompile(`^\d{2}[- ]\d{3}/\d{4}$`),
		Keywords:    []string{"abgabenkontonummer", "abgabenkonto", "stnr", "st.nr.", "st.-nr."},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Validation:  "Luhn check digit is valid",
	})
}

func validATTaxNumber(s string) bool {
	return len(s) == 9 && s != "000000000" && Luhn(s)
}

var atSVNRWeights = [10]int{3, 7, 9, 0, 5, 8, 4, 2, 1, 6}

func validATSocialInsurance(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 || d[0] == 0 {
		return false
	}
	day, month, yy := d[4]*10+d[5], d[6]*10+d[7], d[8]*10+d[9]
	switch {
	case month >= 13 && month <= 15:
		if day < 1 || day > 31 {
			return false
		}
	case !ValidDate(1900+yy, month, day) && !ValidDate(2000+yy, month, day):
		return false
	}
	sum := 0
	for i, w := range atSVNRWeights {
		sum += d[i] * w
	}
	check := sum % 11
	return check != 10 && check == d[3]
}
