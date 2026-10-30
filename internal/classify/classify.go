// Package classify reads a field name, such as a column or property name, and says which kind of personal
// data it holds, flagging the special categories of GDPR Art. 9. Words come from words/<language>.txt and
// from the detectors' concept keywords, in all 24 EU languages.
package classify

import (
	"embed"
	"fmt"
	"path"
	"strings"
	"unicode"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/textfold"
)

//go:embed words/*.txt
var wordFiles embed.FS

// order lists the categories: special categories always win; otherwise the longest matching phrase does,
// and on a tie the earlier category.
var order = []domain.DataCategory{
	domain.CategorySpecial, domain.CategoryIdentifier, domain.CategoryFinancial, domain.CategoryBirth,
	domain.CategoryName, domain.CategoryContact, domain.CategoryAddress, domain.CategoryLocation,
	domain.CategoryOnline, domain.CategoryDemographic,
}

// conceptCategory maps the detectors' keyword concepts to inventory categories.
var conceptCategory = map[detectors.Concept]domain.DataCategory{
	detectors.Name: domain.CategoryName, detectors.Phone: domain.CategoryContact, detectors.Email: domain.CategoryContact,
	detectors.Address: domain.CategoryAddress, detectors.Postcode: domain.CategoryAddress,
	detectors.NationalID: domain.CategoryIdentifier, detectors.TaxID: domain.CategoryIdentifier,
	detectors.SocialSecurity: domain.CategoryIdentifier, detectors.HealthInsurance: domain.CategoryIdentifier,
	detectors.Passport: domain.CategoryIdentifier, detectors.IDCard: domain.CategoryIdentifier,
	detectors.DrivingLicence: domain.CategoryIdentifier, detectors.LicencePlate: domain.CategoryIdentifier,
	detectors.BankAccount: domain.CategoryFinancial, detectors.Card: domain.CategoryFinancial,
	detectors.BirthDate: domain.CategoryBirth, detectors.Location: domain.CategoryLocation,
}

// tooVague are concept keywords that help in text but say little as a field name on their own.
var tooVague = map[string]bool{"card": true, "credit": true, "debit": true, "pan": true, "tel": true, "tin": true, "mail": true, "konto": true, "carta": true, "karta": true, "kort": true, "karte": true, "kaart": true, "location": true, "standort": true, "posizione": true, "plats": true, "vieta": true, "uid": true}

type phrase struct {
	tokens []string
	joined string
	class  domain.Classification
}

var (
	phrases  = load()
	excluded []phrase
)

func load() map[domain.DataCategory][]phrase {
	out := map[domain.DataCategory][]phrase{}
	add := func(word string, class domain.Classification) {
		toks := tokenize(word)
		if len(toks) == 0 {
			return
		}
		p := phrase{tokens: toks, joined: strings.Join(toks, ""), class: class}
		out[class.Category] = append(out[class.Category], p)
	}
	special := map[string]domain.SpecialCategory{}
	for _, s := range domain.SpecialCategories {
		special[string(s)] = s
	}
	for _, lang := range detectors.Languages {
		data, err := wordFiles.ReadFile(path.Join("words", lang+".txt"))
		if err != nil {
			panic("classify: missing word file for " + lang)
		}
		for n, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, list, ok := strings.Cut(line, ":")
			if !ok {
				panic(fmt.Sprintf("classify: words/%s.txt line %d has no colon", lang, n+1))
			}
			key = strings.TrimSpace(key)
			for _, w := range strings.Split(list, ",") {
				w = strings.TrimSpace(w)
				switch {
				case w == "":
				case key == "not":
					toks := tokenize(w)
					excluded = append(excluded, phrase{tokens: toks, joined: strings.Join(toks, "")})
				case special[key] != "":
					add(w, domain.Classification{Category: domain.CategorySpecial, Special: special[key], Matched: w})
				default:
					add(w, domain.Classification{Category: domain.DataCategory(key), Matched: w})
				}
			}
		}
	}
	for concept, cat := range conceptCategory {
		for _, w := range detectors.ConceptWords(concept) {
			if !tooVague[w] {
				add(w, domain.Classification{Category: cat, Matched: w})
			}
		}
	}
	return out
}

// Field says what a field name holds. ok is false when the name says nothing about personal data.
func Field(name string) (domain.Classification, bool) {
	toks := tokenize(name)
	if len(toks) == 0 {
		return domain.Classification{}, false
	}
	for _, p := range excluded {
		if p.matches(toks) {
			return domain.Classification{}, false
		}
	}
	for _, p := range phrases[domain.CategorySpecial] {
		if p.matches(toks) {
			return p.class, true
		}
	}
	var best phrase
	for _, cat := range order[1:] {
		for _, p := range phrases[cat] {
			if len(p.joined) > len(best.joined) && p.matches(toks) {
				best = p
			}
		}
	}
	return best.class, best.joined != ""
}

// matches reports whether the phrase appears in the tokens, either word by word or written together
// (first_name and firstname both match "first name").
func (p phrase) matches(toks []string) bool {
	for i := range toks {
		if i+len(p.tokens) <= len(toks) && equal(toks[i:i+len(p.tokens)], p.tokens) {
			return true
		}
		joined := ""
		for j := i; j < len(toks) && len(joined) < len(p.joined); j++ {
			joined += toks[j]
			if joined == p.joined {
				return true
			}
		}
	}
	return false
}

func equal(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// tokenize splits a name into folded words at underscores, dashes, dots, spaces and camelCase humps, and
// drops a plural "s" from English-looking words so "allergies" and "diagnoses" still match their entries.
func tokenize(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, textfold.Fold(string(cur)))
			cur = cur[:0]
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(runes[i-1]) || i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1])):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}
