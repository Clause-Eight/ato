package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Dutch identity card document number (documentnummer identiteitskaart), the same scheme as the passport: nine
// characters, two letters, six letters or digits and a digit. The letter O is never used, so that it cannot be
// read as a zero. There is no check digit outside the machine-readable zone. Source: RvIG, "Kenmerken paspoorten
// en identiteitskaarten model 2014",
// https://www.rvig.nl/sites/default/files/2023-09/Kenmerkenbrochure%20paspoorten%20en%20identiteitskaarten%20model%202014%20EN.pdf
func init() {
	Register(Detector{
		Name:         "nl-id-card",
		Label:        "Dutch identity card document number",
		Country:      "NL",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[A-NP-Z]{2}[0-9A-NP-Z]{6}[0-9]`),
		Keywords:     []string{"identiteitskaartnummer", "nummer identiteitskaart", "documentnummer identiteitskaart", "id-kaartnummer"},
		Concepts:     []Concept{IDCard},
		Base:         domain.Low,
		Validation:   "letters (never O) and digits are in their places; there is no control digit",
		NeedsKeyword: true,
		Needs:        Needs{Check: hasNLDocumentStart},
	})
}

const nlDocumentLetters = "ABCDEFGHIJKLMNPQRSTUVWXYZ"

// hasNLDocumentStart looks for two capitals other than O at the start of a word, followed by a capital or a digit.
func hasNLDocumentStart(line string) bool {
	for i := 0; i+2 < len(line); i++ {
		if i > 0 && isWordByte(line[i-1]) || strings.IndexByte(nlDocumentLetters, line[i]) < 0 || strings.IndexByte(nlDocumentLetters, line[i+1]) < 0 {
			continue
		}
		if c := line[i+2]; c >= '0' && c <= '9' || strings.IndexByte(nlDocumentLetters, c) >= 0 {
			return true
		}
	}
	return false
}
