package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Irish Personal Public Service Number (PPSN, PPS No): seven digits, a check letter and, on numbers issued since
// 2013, a second letter (A or B for people, H for non-individuals); older numbers may carry W (married women,
// until 1999) or T or X, which do not count in the check. The check letter is the sum of the digits weighted 8 to 2,
// plus the second letter's place in the alphabet (A=1) weighted 9, taken mod 23: 1-22 is A-V and 0 is W.
// Sources: https://en.wikipedia.org/wiki/Personal_Public_Service_Number and the Revenue TIN sheet published by the
// OECD, https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Ireland-TIN.pdf
func init() {
	Register(Detector{
		Name:        "ie-ppsn",
		Label:       "Irish personal public service number (PPSN)",
		Country:     "IE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{7}[A-W][ABHWTX]?`),
		Validate:    validIEPPSN,
		Keywords: []string{"ppsn", "pps", "pps no", "pps number", "ppsno", "personal public service",
			"uimhir phearsanta seirbhíse poiblí"},
		Concepts:   []Concept{NationalID, TaxID, SocialSecurity},
		Base:       domain.Low,
		Validation: "check letter is valid",
		Needs:      Needs{Check: hasDigitThenCapital},
	})
}

func validIEPPSN(s string) bool {
	if len(s) != 8 && len(s) != 9 {
		return false
	}
	d, ok := Digits(s[:7])
	if !ok {
		return false
	}
	sum := 0
	for i, x := range d {
		sum += x * (8 - i)
	}
	if len(s) == 9 {
		switch s[8] {
		case 'A', 'B', 'H':
			sum += int(s[8]-'A'+1) * 9
		case 'W', 'T', 'X':
		default:
			return false
		}
	}
	return ieCheckLetter(sum%23) == s[7]
}

func ieCheckLetter(rem int) byte {
	if rem == 0 {
		return 'W'
	}
	return byte('A' + rem - 1)
}

// hasDigitThenCapital reports whether a digit is directly followed by a capital letter, as in a PPSN.
func hasDigitThenCapital(line string) bool {
	for i := 1; i < len(line); i++ {
		if c := line[i]; c >= 'A' && c <= 'W' && line[i-1] >= '0' && line[i-1] <= '9' {
			return true
		}
	}
	return false
}
