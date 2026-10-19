package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Cyprus identity card number (αριθμός δελτίου ταυτότητας), issued by the Civil Registry and Migration
// Department. The number has no published check digit and its length is not published either: cards seen in
// document-verification references carry six or seven digits, so six to eight digits are accepted, and only
// on a line that names the card: a bare number of that length is too common to trust a keyword on the line
// before. Sources:
// https://en.wikipedia.org/wiki/Cypriot_identity_card and
// https://learn.microsoft.com/en-us/purview/sit-defn-cyprus-identity-card (format only).
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
