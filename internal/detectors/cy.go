package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Cyprus identity card number (αριθμός δελτίου ταυτότητας), issued by the Civil Registry and Migration
// Department. The number has no published check digit and its length is not published either: cards seen in
// document-verification references carry six or seven digits, so six to eight digits are accepted, and only
// on a line that names the card: a bare number of that length is too common to trust a keyword on the line
// before. Sources:
// https://en.wikipedia.org/wiki/Cypriot_identity_card and
// https://learn.microsoft.com/en-us/purview/sit-defn-cyprus-identity-card (format only).
//
// Cyprus tax identification code (TIC, αριθμός φορολογικής ταυτότητας): eight digits and an upper-case check
// letter. Digits in the first, third, fifth and seventh places are replaced by 1, 0, 5, 7, 9, 13, 15, 17, 19, 21
// (for 0 to 9), the other four count as themselves, and the sum mod 26 picks the letter, A for 0 to Z for 25. The
// codes of people start with 0 or 9, and those issued by the Tax For All system since 27 March 2023 start from
// 60000000; codes starting with 1, 3, 4 or 5 belong to companies and are left out. The CY-prefixed VAT form is
// left to eu-vat. Sources: the Cyprus TIN sheet published by the OECD (format and the 2023 range),
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/Cyprus-TIN.pdf
// and the European Commission's TIN-on-the-Web syntax check, checkTinService (check letter and first digits),
// https://taxation-customs.ec.europa.eu/online-services/online-services-and-databases-taxation/taxpayer-identification-number-tin_en
func init() {
	Register(Detector{
		Name:         "cy-id-card",
		Label:        "Cyprus identity card number",
		Country:      "CY",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`\d{6,8}`),
		Keywords:     []string{"αριθμός δελτίου ταυτότητας", "αρ. δελτίου ταυτότητας", "αριθμός ταυτότητας", "αρ. ταυτότητας", "αδτ", "α.δ.τ.", "cyprus id", "cypriot id", "cyprus identity card", "kimlik numarası", "kimlik numarasi", "kimlik no"},
		Base:         domain.Low,
		Validation:   "six to eight digits on a line naming the card; there is no control digit",
		NeedsKeyword: true,
		Needs:        Needs{Check: cyNamesCard},
	})
	Register(Detector{
		Name:        "cy-tic",
		Label:       "Cyprus tax identification code (TIC)",
		Country:     "CY",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[069]\d{7}[A-Z]`),
		Validate:    validCYTIC,
		Keywords:    []string{"tic", "tax identification code", "αριθμός φορολογικής ταυτότητας", "αρ. φορολογικής ταυτότητας", "φορολογική ταυτότητα", "αφτ", "α.φ.τ."},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Validation:  "first digit and check letter are valid",
		Needs:       Needs{Check: digitThenCapital},
	})
}

// cyTICOdd replaces each digit in an odd place (first, third, ...) of a TIC.
var cyTICOdd = [10]int{1, 0, 5, 7, 9, 13, 15, 17, 19, 21}

func validCYTIC(s string) bool {
	if len(s) != 9 || s[8] < 'A' || s[8] > 'Z' {
		return false
	}
	d, ok := Digits(s[:8])
	if !ok || d[0] != 0 && d[0] != 6 && d[0] != 9 {
		return false
	}
	sum := 0
	for i, x := range d {
		if i%2 == 0 {
			sum += cyTICOdd[x]
		} else {
			sum += x
		}
	}
	return int(s[8]-'A') == sum%26
}

// digitThenCapital is the prefilter for numbers that end in a check letter: a digit directly followed by A-Z.
func digitThenCapital(line string) bool {
	for i := 0; i+1 < len(line); i++ {
		if line[i] >= '0' && line[i] <= '9' && line[i+1] >= 'A' && line[i+1] <= 'Z' {
			return true
		}
	}
	return false
}

// cyCardWords are the fragments of the card's names, in the cases a line is likely to use.
var cyCardWords = []string{"ταυτότητ", "Ταυτότητ", "ΤΑΥΤΟΤΗΤ", "ταυτοτητ", "αδτ", "ΑΔΤ", "α.δ.τ", "Α.Δ.Τ", "yprus", "ypriot", "YPRUS", "YPRIOT", "imlik", "IMLIK"}

// cyNamesCard is the prefilter: the line itself must name the card.
func cyNamesCard(line string) bool {
	for _, w := range cyCardWords {
		if strings.Contains(line, w) {
			return true
		}
	}
	return false
}
