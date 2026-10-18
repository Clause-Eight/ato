package detectors

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
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

// Fold prepares text for keyword matching: lower case, Latin diacritics and Greek accents removed, final sigma
// made ordinary, and underscores and dashes turned into spaces. Cyrillic is only lower-cased.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.TrimSpace(s) {
		r = unicode.ToLower(r)
		if r == '_' || r == '-' {
			b.WriteByte(' ')
			continue
		}
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		if f, ok := folds[r]; ok {
			b.WriteString(f)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// folds covers the letters of the EU's Latin and Greek alphabets that carry marks.
var folds = func() map[rune]string {
	m := map[rune]string{'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o", 'đ': "d", 'ł': "l", 'ħ': "h", 'ı': "i", 'ς': "σ"}
	groups := map[string]string{
		"a": "àáâãäåāăą", "c": "çćčċ", "d": "ď", "e": "èéêëēėęěĕ", "g": "ğġģ", "i": "ìíîïīįĭ", "k": "ķ",
		"l": "ĺļľ", "n": "ñńņň", "o": "òóôõöōőŏ", "r": "ŕř", "s": "śšşș", "t": "ťţț", "u": "ùúûüūůűųŭ",
		"y": "ýÿ", "z": "źżž",
		"α": "ά", "ε": "έ", "η": "ή", "ι": "ίϊΐ", "ο": "ό", "υ": "ύϋΰ", "ω": "ώ",
	}
	for base, marked := range groups {
		for _, r := range marked {
			m[r] = base
		}
	}
	return m
}()
