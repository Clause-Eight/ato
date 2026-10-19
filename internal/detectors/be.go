package detectors

import (
	"regexp"
	"strconv"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Belgian national register number (rijksregisternummer, numéro de registre national): eleven digits YY MM DD SSS
// CC, usually printed YY.MM.DD-SSS.CC. SSS counts the people born that day, 001-997 odd for men and 002-998 even
// for women. CC is 97 minus the first nine digits mod 97; for people born from 2000 a 2 is put in front of the nine
// digits first. The birth date may be incomplete: month and day can both be 00. The bis number of the Crossroads
// Bank for Social Security has the same structure with 40 added to the month (sex known) or 20 (sex unknown).
// Sources: National Register instruction IT000,
// https://www.rijksregister.fgov.be/sites/default/files/documents/nl/rijksregister/onderrichtingen/IT-lijst/IT000_Rijksregisternummer.pdf
// and the KSZ-BCSS IdentifyPerson user guide,
// https://www.ksz-bcss.fgov.be/sites/default/files/assets/diensten_en_support/documentatie/identifyperson_gebruikershandleiding.doc
func init() {
	Register(Detector{
		Name:        "be-national-register",
		Label:       "Belgian national register number (rijksregisternummer, numéro de registre national)",
		Country:     "BE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{2}\.?\d{2}\.?\d{2}[-. ]?\d{3}[-. ]?\d{2}`),
		Normalize:   DigitsOnly,
		Validate:    validBENationalRegister,
		Formatted:   regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}[- ]\d{3}[.-]\d{2}$`),
		Keywords: []string{"rijksregisternummer", "rijksregister", "registre national", "numéro de registre national",
			"numéro national", "nationaal nummer", "nationalregisternummer", "insz", "niss", "inss", "ssin",
			"bisnummer", "bis-nummer", "numéro bis", "rrn"},
		Concepts:   []Concept{NationalID, SocialSecurity, TaxID},
		Base:       domain.Low,
		Validation: "birth date and mod-97 check are valid",
	})
}

func validBENationalRegister(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 {
		return false
	}
	serial := d[6]*100 + d[7]*10 + d[8]
	if serial == 0 || serial == 999 {
		return false
	}
	body, err := strconv.Atoi(s[:9])
	if err != nil {
		return false
	}
	check := d[9]*10 + d[10]
	year := 0
	switch check {
	case 97 - body%97:
		year = 1900
	case 97 - (2_000_000_000+body)%97:
		year = 2000
	default:
		return false
	}
	year += d[0]*10 + d[1]
	month, day := d[2]*10+d[3], d[4]*10+d[5]
	switch {
	case month >= 40:
		month -= 40
	case month >= 20:
		month -= 20
	}
	if month > 12 || day > 31 {
		return false
	}
	if month == 0 || day == 0 {
		return true
	}
	return ValidDate(year, month, day)
}
