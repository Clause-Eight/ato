package detectors

import (
	"embed"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Person names in a naming context: a field or key such as first_name, nachname, prenume or nazwisko, then an
// assigned or quoted value of one to three capitalised words, each found in the name list. The list holds the
// most common given names and surnames per member state, compiled from Wikidata (CC0); internal/detectors/names/
// SOURCES.md records how. Free text ("signed by Maria") is out of scope: without a key on the same line nothing
// is reported. A name with a prefix glued in camelCase (customerFirstName) is not recognised, and a rare name
// that is not in the list is missed. Off by default and only reported in its key context.
func init() {
	Register(Detector{
		Name:         "person-name",
		Label:        "Person name in a name field",
		Country:      domain.CrossBorder,
		Pattern:      regexp.MustCompile(`(?i:` + keyAlternatives() + `)\\?["']?\]?(?:\s*(?:=>|:=|[:=])\s*|>)\\?["']?` + nameValue),
		Normalize:    nameValueOnly,
		Validate:     validPersonName,
		Keywords:     nameKeywords(),
		Concepts:     []Concept{Name},
		Base:         domain.Low,
		Validation:   "a name field holds given names or surnames from the name list",
		TrimGroups:   true,
		Needs:        Needs{Check: mayHaveNameKey},
		NeedsKeyword: true,
	})
}

const (
	nameWord  = `\p{Lu}\p{Ll}+(?:-\p{Lu}\p{Ll}+)?`
	nameValue = nameWord + `(?: ` + nameWord + `){0,2}`
)

// nameKeys are the field names, lower case; a space stands for an optional underscore, dash or space.
var nameKeys = []string{
	"first name", "last name", "given name", "given names", "family name", "middle name", "sur name", "fore name",
	"full name", "vor name", "nach name", "familien name", "pre nume", "nume", "nume complet", "prénom", "prenom",
	"nom", "nom de famille", "nom complet", "nombre", "nombre completo", "apellido", "apellidos", "imię", "imie",
	"nazwisko", "imię i nazwisko", "cognome", "nome", "nome completo", "voor naam", "achter naam", "familie naam",
	"kereszt név", "kereszt nev", "vezeték név", "vezetek nev", "utó név", "uto nev", "teljes név", "for navn",
	"efter navn", "för namn", "efter namn", "etu nimi", "suku nimi", "jméno", "jmeno", "příjmení", "prijmeni", "meno",
	"priezvisko", "ime", "prezime", "priimek", "vardas", "pavardė", "pavarde", "vārds", "uzvārds", "uzvards",
	"ees nimi", "perekonna nimi", "apelido", "sobrenome", "ainm", "sloinne", "isem", "kunjom", "име", "фамилия",
	"презиме", "όνομα", "ονομα", "επώνυμο", "επωνυμο", "ονοματεπώνυμο",
}

func keyAlternatives() string {
	alts := make([]string, len(nameKeys))
	for i, k := range nameKeys {
		words := strings.Fields(k)
		for j, w := range words {
			words[j] = regexp.QuoteMeta(w)
		}
		alts[i] = strings.Join(words, `[_ -]?`)
	}
	return strings.Join(alts, "|")
}

// nameKeywords are the keys written with and without the separator, so the keyword check finds first_name,
// first-name and firstName alike.
func nameKeywords() []string {
	var out []string
	for _, k := range nameKeys {
		out = append(out, k)
		if strings.Contains(k, " ") {
			out = append(out, strings.ReplaceAll(k, " ", ""))
		}
	}
	return out
}

// nameKeyForms are the keys in every spelling a line can hold them, for the prefilter.
var nameKeyForms = func() []string {
	var out []string
	for _, k := range nameKeys {
		forms := []string{""}
		for i, w := range strings.Fields(k) {
			if i == 0 {
				forms[0] = w
				continue
			}
			var next []string
			for _, f := range forms {
				for _, sep := range []string{"", "_", "-", " "} {
					next = append(next, f+sep+w)
				}
			}
			forms = next
		}
		out = append(out, forms...)
	}
	return out
}()

//go:embed names/*.txt
var nameFiles embed.FS

// personNames is the set of names from every list, as written (capitalised).
var personNames = func() map[string]bool {
	set := map[string]bool{}
	entries, err := nameFiles.ReadDir("names")
	if err != nil {
		panic("detectors: name lists missing")
	}
	for _, e := range entries {
		data, err := nameFiles.ReadFile("names/" + e.Name())
		if err != nil {
			panic("detectors: cannot read name list " + e.Name())
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				set[line] = true
			}
		}
	}
	return set
}()

var trailingName = regexp.MustCompile(nameValue + `$`)

// nameValueOnly keeps the name and drops the key in front of it. Without a key and a separator in front there is
// no name field, and the result is empty.
func nameValueOnly(s string) string {
	loc := trailingName.FindStringIndex(s)
	if loc == nil || loc[0] == 0 || !strings.ContainsRune(` "'=:>`, rune(s[loc[0]-1])) {
		return ""
	}
	return s[loc[0]:]
}

func validPersonName(s string) bool {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '-' })
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		if !personNames[p] {
			return false
		}
	}
	return true
}

// KnownName reports whether a word is a given name or surname in the name list, ignoring case.
func KnownName(word string) bool {
	r, n := utf8.DecodeRuneInString(word)
	if n == 0 {
		return false
	}
	return personNames[string(unicode.ToUpper(r))+strings.ToLower(word[n:])]
}

// mayHaveNameKey is the prefilter: somewhere on the line a key is followed by a separator and a capital letter.
func mayHaveNameKey(line string) bool {
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c != ':' && c != '=' && c != '>' {
			continue
		}
		k := i + 1
		for k < len(line) && strings.IndexByte(" \t=>\\\"'", line[k]) >= 0 {
			k++
		}
		if k >= len(line) {
			return false
		}
		if r, _ := utf8.DecodeRuneInString(line[k:]); !unicode.IsUpper(r) {
			continue
		}
		j := i
		for j > 0 && strings.IndexByte(" \t:=\\\"']", line[j-1]) >= 0 {
			j--
		}
		key := strings.ToLower(line[max(0, j-40):j])
		for _, f := range nameKeyForms {
			if strings.HasSuffix(key, f) {
				return true
			}
		}
	}
	return false
}
