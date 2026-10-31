package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Street address: a street-type word in one of the EU languages next to a capitalised street name and a house
// number, in the order each language writes it: "Strada Lalelelor 12", "ul. Kwiatowa 3", "Calle Mayor, 7",
// "Via Roma 1", "10 rue de la Paix", "Hauptstraße 5", "Kerkstraat 12", "Fő utca 1", "Brīvības iela 12",
// "Gedimino pr. 9", "12 Main Street". The street word is itself the context, so no keyword is required, but
// the base is low: at the default level an address word must be nearby. Street-type vocabulary from the
// UPU's postal addressing systems by country (https://www.upu.int/en/Postal-Solutions/Programmes-Services/Addressing-Solutions).
func init() {
	Register(Detector{
		Name:        "street-address",
		Label:       "Street address (street and house number)",
		Country:     "EU",
		OnByDefault: false,
		Pattern:     streetPattern,
		Validate:    validStreet,
		Keywords:    []string{"street_address", "address_line", "addressline", "streetaddress"},
		Concepts:    []Concept{Address},
		Base:        domain.Low,
		Validation:  "a street-type word, a street name and a house number",
		Needs:       Needs{Check: hasStreetWord},
	})
}

// Street-type words. Words that are common English (via, place, corso, largo, plaza) must be capitalised.
var (
	streetBefore = []string{
		"strada", "str.", "bulevardul", "bd.", "bdul.", "calea", "aleea", "șoseaua", "soseaua", "splaiul", "piața", "piata", "intrarea",
		"ul.", "ulica", "al.", "aleja", "plac", "os.", "ulice", "nám.", "náměstí", "námestie", "trg",
		"viale", "piazza", "vicolo", "calle", "c/", "avenida", "avda.", "av.", "paseo", "carrer", "camino", "ronda",
		"rua", "travessa", "praça", "praca", "alameda", "estrada",
		"rue", "avenue", "boulevard", "chemin", "impasse", "allée", "quai",
		"ул.", "улица", "бул.", "булевард", "οδός", "λεωφόρος", "λεωφ.", "triq", "sráid",
	}
	streetBeforeCapital = []string{"Via", "Place", "Corso", "Largo", "Plaza", "Route"}
	streetNumberFirst   = []string{"rue", "avenue", "boulevard", "bd", "chemin", "impasse", "allée", "place", "quai", "route"}
	streetAfter         = []string{
		"utca", "út", "u.", "tér", "körút", "krt.", "köz", "iela", "gatvė", "g.", "pr.", "prospektas",
		"ulica", "cesta", "trg", "tänav", "mnt", "pst", "maantee", "puiestee", "Straße", "Strasse", "Allee", "Weg", "Gasse",
		"Platz", "laan", "straat", "weg",
	}
	streetEnglish = []string{"Street", "Road", "Avenue", "Lane", "Drive", "Way", "Close", "Square", "Terrace", "Crescent", "Court", "Gardens"}
	streetSuffix  = []string{
		"straße", "strasse", "str.", "gasse", "allee", "weg", "platz", "ufer",
		"straat", "laan", "gracht", "plein", "steeg", "kade", "singel",
		"gade", "vej", "gatan", "vägen", "gränd", "torget",
		"katu", "tie", "kuja", "polku", "väylä",
	}
)

// capitalisedEither turns "strada" into "[Ss]trada", so a street word may start a sentence.
func capitalisedEither(words []string) string {
	parts := make([]string, 0, len(words))
	for _, w := range words {
		r := []rune(w)
		first := string(r[0])
		up := strings.ToUpper(first)
		head := regexp.QuoteMeta(first)
		if up != first {
			head = "[" + up + first + "]"
		}
		parts = append(parts, head+regexp.QuoteMeta(string(r[1:])))
	}
	return "(?:" + strings.Join(parts, "|") + ")"
}

func quoted(words []string) string {
	parts := make([]string, 0, len(words))
	for _, w := range words {
		parts = append(parts, regexp.QuoteMeta(w))
	}
	return "(?:" + strings.Join(parts, "|") + ")"
}

var streetPattern = func() *regexp.Regexp {
	particle := `(?:(?:de|la|du|des|del|della|di|da|do|dos|das|le|les|lui|van|von|der|den|il|tal|ta) |d'|l'|il-|tal-|ta')`
	word := `\p{Lu}[\p{L}'’-]*`
	name := particle + `*` + word + `(?: ` + particle + `*` + word + `){0,3}`
	number := `(?:(?:nr|no|n)\.? ?|n° ?)?[1-9]\d{0,3}[a-zA-Z]?(?:/\d{1,4})?`
	before := `(?:` + capitalisedEither(streetBefore) + `|` + quoted(streetBeforeCapital) + `) ` + name + `,? ` + number
	numberFirst := number + `,? ` + capitalisedEither(streetNumberFirst) + ` ` + name
	after := name + ` ` + quoted(streetAfter) + `,? ` + number
	english := number + ` ` + name + ` ` + quoted(streetEnglish)
	suffix := `\p{Lu}[\p{L}-]*` + capitalisedEither(streetSuffix) + `,? ` + number
	return regexp.MustCompile(before + `|` + numberFirst + `|` + after + `|` + english + `|` + suffix)
}()

// streetStems are the lower-case street words; short ones (four letters or fewer) must start a word.
var streetStems, shortStreetStems = func() ([]string, []string) {
	var long, short []string
	for _, w := range streetSuffix {
		long = append(long, strings.ToLower(w))
	}
	for _, list := range [][]string{streetBefore, streetBeforeCapital, streetNumberFirst, streetAfter, streetEnglish} {
		for _, w := range list {
			w = strings.ToLower(w)
			if len([]rune(w)) <= 4 {
				short = append(short, w)
			} else {
				long = append(long, w)
			}
		}
	}
	return unique(long), unique(short)
}()

// hasStreetWord is the prefilter: a capitalised word, a house number (one to four digits on their own) and a
// street-type word somewhere in the line, ignoring case.
func hasStreetWord(line string) bool {
	if !hasCapitalisedWord(line) || !hasShortNumber(line) {
		return false
	}
	lower := strings.ToLower(line)
	for _, w := range streetStems {
		if strings.Contains(lower, w) {
			return true
		}
	}
	for _, w := range shortStreetStems {
		for from := 0; ; {
			i := strings.Index(lower[from:], w)
			if i < 0 {
				break
			}
			i += from
			if i == 0 || !(lower[i-1] >= 'a' && lower[i-1] <= 'z') {
				return true
			}
			from = i + 1
		}
	}
	return false
}

// hasCapitalisedWord looks for a capital followed by a small letter; any non-ASCII byte may be either.
func hasCapitalisedWord(line string) bool {
	for i := 0; i+1 < len(line); i++ {
		c, n := line[i], line[i+1]
		if (c >= 'A' && c <= 'Z' || c >= 0x80) && (n >= 'a' && n <= 'z' || n >= 0x80) {
			return true
		}
	}
	return false
}

func hasShortNumber(line string) bool {
	run := 0
	for i := 0; i <= len(line); i++ {
		if i < len(line) && isASCIIDigit(line[i]) {
			run++
			continue
		}
		if run >= 1 && run <= 4 {
			return true
		}
		run = 0
	}
	return false
}

var houseNumber = regexp.MustCompile(`(?:^|[ ,.°])([1-9]\d{0,3})[a-zA-Z]?(?:/\d{1,4})?(?:,|$| )`)

// validStreet requires a house number from 1 to 9999 at the start or the end of the candidate.
func validStreet(s string) bool {
	return houseNumber.MatchString(s) || houseNumber.MatchString(" "+s)
}
