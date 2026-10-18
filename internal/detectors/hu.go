package detectors

import (
	"regexp"
	"time"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Hungarian social security number (TAJ): nine digits; digits 1-8 weighted 3,7,3,7…, sum mod 10 is digit 9.
// Hungarian tax ID (adóazonosító jel): ten digits starting with 8; digits 2-6 are days since 1 January 1867;
// digit 10 is sum(digit_i * i, i = 1..9) mod 11, and a remainder of 10 is never issued.
func init() {
	Register(Detector{
		Name:        "hu-taj",
		Label:       "Hungarian social security number (TAJ)",
		Country:     "HU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{3}[ -]?\d{3}[ -]?\d{3}`),
		Normalize:   DigitsOnly,
		Validate:    validTAJ,
		Keywords:    []string{"taj", "társadalombiztosítási", "tarsadalombiztositasi", "social_security", "socialsecurity", "health_insurance"},
		Base:        domain.Low,
		Validation:  "check digit is valid",
	})
	Register(Detector{
		Name:        "hu-tax-id",
		Label:       "Hungarian tax ID (adóazonosító jel)",
		Country:     "HU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`8\d{9}`),
		Validate:    validHUTaxID,
		Keywords:    []string{"adóazonosító", "adoazonosito", "tax_id", "taxid", "tax_number", "taxnumber"},
		Base:        domain.Low,
		Validation:  "birth-date part and check digit are valid",
	})
}

func validTAJ(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 {
		return false
	}
	sum := 0
	for i := 0; i < 8; i++ {
		if i%2 == 0 {
			sum += d[i] * 3
		} else {
			sum += d[i] * 7
		}
	}
	return sum%10 == d[8]
}

var huTaxEpoch = time.Date(1867, 1, 1, 0, 0, 0, 0, time.UTC)

func validHUTaxID(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 || d[0] != 8 {
		return false
	}
	days := d[1]*10000 + d[2]*1000 + d[3]*100 + d[4]*10 + d[5]
	born := huTaxEpoch.AddDate(0, 0, days)
	if days == 0 || born.Year() > 2100 {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += d[i] * (i + 1)
	}
	check := sum % 11
	return check != 10 && check == d[9]
}
