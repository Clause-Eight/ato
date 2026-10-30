package detectors

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/textfold"
)

// Concept names a kind of personal data that has keywords in every language. A detector lists its concepts;
// the words come from keywords/<language>.txt.
type Concept string

const (
	NationalID      Concept = "national-id"
	TaxID           Concept = "tax-id"
	SocialSecurity  Concept = "social-security"
	HealthInsurance Concept = "health-insurance"
	Phone           Concept = "phone"
	Email           Concept = "email"
	BankAccount     Concept = "bank-account"
	Card            Concept = "card"
	VAT             Concept = "vat"
	Passport        Concept = "passport"
	IDCard          Concept = "id-card"
	BirthDate       Concept = "birth-date"
	Address         Concept = "address"
	Postcode        Concept = "postcode"
	DrivingLicence  Concept = "driving-licence"
	LicencePlate    Concept = "licence-plate"
	Location        Concept = "location"
	Name            Concept = "name"
)

// Languages are the 24 official EU languages, by ISO 639-1 code. Each has a file in keywords/.
var Languages = []string{"bg", "cs", "da", "de", "el", "en", "es", "et", "fi", "fr", "ga", "hr", "hu", "it", "lt", "lv", "mt", "nl", "pl", "pt", "ro", "sk", "sl", "sv"}

//go:embed keywords/*.txt
var keywordFiles embed.FS

// conceptWords maps each concept to its folded words in every language. It is built before any init runs.
var conceptWords = loadKeywords()

func loadKeywords() map[Concept][]string {
	words := map[Concept][]string{}
	for _, lang := range Languages {
		data, err := keywordFiles.ReadFile(path.Join("keywords", lang+".txt"))
		if err != nil {
			panic("detectors: missing keyword file for " + lang)
		}
		for n, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			concept, list, ok := strings.Cut(line, ":")
			if !ok {
				panic(fmt.Sprintf("detectors: keywords/%s.txt line %d has no colon", lang, n+1))
			}
			c := Concept(strings.TrimSpace(concept))
			for _, w := range strings.Split(list, ",") {
				if w = Fold(w); w != "" {
					words[c] = append(words[c], w)
				}
			}
		}
	}
	for c, ws := range words {
		words[c] = unique(ws)
	}
	return words
}

// ConceptWords returns the folded words for a concept in every language.
func ConceptWords(c Concept) []string { return conceptWords[c] }

// KnownConcepts lists every concept that has words, sorted.
func KnownConcepts() []Concept {
	out := make([]Concept, 0, len(conceptWords))
	for c := range conceptWords {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func unique(ws []string) []string {
	seen := map[string]bool{}
	out := ws[:0]
	for _, w := range ws {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// Fold prepares text for keyword matching; see textfold.Fold.
func Fold(s string) string { return textfold.Fold(s) }
