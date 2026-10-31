package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Passport number as printed on the data page, outside the machine-readable zone (the MRZ is left to mrz). Each
// issuing state chooses its own layout, so the detector keeps a table of them and a value is valid when it fits
// one. There is no control digit outside the MRZ, except the German serial: it may be written with its ICAO 9303
// check digit (as in de-id-card), and when it is, the digit is verified.
//
// Layouts, from Microsoft Purview's sensitive information type reference
// (https://learn.microsoft.com/en-us/purview/sit-defn-<country>-passport-number, read for each country; format only):
//   - AT one letter, an optional space, seven digits; EE, SK one letter and seven digits; PT one letter and six
//     digits; CY one letter and six to eight digits.
//   - BE two letters and six digits; FI, GR, PL two letters and seven digits; HU two letters and six or seven
//     digits (SK too); SI P, a letter and seven digits.
//   - IE, IT, LV two letters or digits and seven digits; ES two letters or digits, an optional third, six digits;
//     FR two digits, two letters, five digits; LT eight letters or digits; GB (sit-defn-us-uk-passport-number) a
//     letter or digit and eight digits.
//   - BG, DK, HR nine digits; CZ, SE eight digits; RO eight or nine digits; MT seven digits.
//   - DE one of C F G H J K, then eight characters from the digits and C F G H J K L M N P R T V W X Y Z, and the
//     optional check digit; the alphabet is the one in de-id-card's source, "Die alphanumerische Seriennummer im
//     neuen Personalausweis",
//     https://www.marktflecken-villmar.de/fileadmin/villmar/downloads/ausweisrecht/Seriennummer-des-Personalausweises.pdf
//   - NL two letters, six letters or digits, one digit, never the letter O: RvIG, "Kenmerken paspoorten en
//     identiteitskaarten model 2014",
//     https://www.rvig.nl/sites/default/files/2023-09/Kenmerkenbrochure%20paspoorten%20en%20identiteitskaarten%20model%202014%20EN.pdf
//   - CH the 2010 passport only: X and seven digits, quoted from fedpol in "How can you distinguish between a zero and
//     an «O» in a passport?", travelnews.ch, 21 March 2024,
//     https://www.travelnews.ch/english-corner/26038-how-can-you-distinguish-between-a-zero-and-an-o-in-a-passport.html
//
// Not covered: LU (only withdrawn models are described), IS, LI and NO (no published layout), and the Swiss 2022
// passport, whose number fedpol says no longer has a fixed structure. Many layouts are bare runs of digits that look
// like any other number, so the line itself must name a passport (a keyword on the line before is not enough), and
// a value needs at least one digit. Every layout fits seven to ten capitals and digits, which the pattern finds;
// Validate checks them against the table.
func init() {
	Register(Detector{
		Name:          "passport",
		Label:         "Passport number",
		Country:       domain.CrossBorder,
		OnByDefault:   true,
		Pattern:       regexp.MustCompile(`[A-Z] [0-9]{7}|[A-Z0-9]{7,10}`),
		Normalize:     func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:      validPassport,
		Keywords:      []string{"passport no", "passportno", "passportnumber", "passportnr", "passnr", "pass nr", "reisepassnr", "reisepass nr", "n° de passeport", "nr paszportu", "číslo cestovního pasu", "номер на паспорта", "αρ. διαβατηρίου"},
		Concepts:      []Concept{Passport},
		Base:          domain.Low,
		Validation:    "fits an issuing state's passport number layout; there is no control digit, except the German one, verified when written",
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Needs:         Needs{Digits: 1, Check: namesPassport},
	})
}

type passportLayout struct {
	countries string
	re        string
}

var passportLayouts = []passportLayout{
	{"AT EE SK", `[A-Z][0-9]{7}`},
	{"PT", `[A-Z][0-9]{6}`},
	{"CY", `[A-Z][0-9]{6,8}`},
	{"BE", `[A-Z]{2}[0-9]{6}`},
	{"FI GR PL", `[A-Z]{2}[0-9]{7}`},
	{"HU SK", `[A-Z]{2}[0-9]{6,7}`},
	{"SI", `P[A-Z][0-9]{7}`},
	{"IE IT LV", `[A-Z0-9]{2}[0-9]{7}`},
	{"ES", `[A-Z0-9]{2,3}[0-9]{6}`},
	{"FR", `[0-9]{2}[A-Z]{2}[0-9]{5}`},
	{"LT", `[A-Z0-9]{8}`},
	{"GB", `[A-Z0-9][0-9]{8}`},
	{"BG DK HR RO", `[0-9]{9}`},
	{"CZ SE RO", `[0-9]{8}`},
	{"MT", `[0-9]{7}`},
	{"DE", `[CFGHJK][0-9CFGHJKLMNPRTVWXYZ]{8}`},
	{"NL", `[A-NP-Z]{2}[0-9A-NP-Z]{6}[0-9]`},
	{"CH", `X[0-9]{7}`},
}

var passportAnchored = func() *regexp.Regexp {
	alts := make([]string, len(passportLayouts))
	for i, l := range passportLayouts {
		alts[i] = l.re
	}
	return regexp.MustCompile(`^(?:` + strings.Join(alts, `|`) + `)$`)
}()

// dePassportFirst are the first letters of a German passport serial; de-id-card uses the others.
const dePassportFirst = "CFGHJK"

func validPassport(s string) bool {
	if len(s) < 7 || len(s) > 10 || !strings.ContainsAny(s, "0123456789") {
		return false
	}
	if len(s) == 10 {
		return validDEPassportWithCheck(s)
	}
	return passportAnchored.MatchString(s)
}

// validDEPassportWithCheck accepts a German passport serial followed by its ICAO 9303 check digit.
func validDEPassportWithCheck(s string) bool {
	if strings.IndexByte(dePassportFirst, s[0]) < 0 || s[9] < '0' || s[9] > '9' {
		return false
	}
	for i := 1; i < 9; i++ {
		if strings.IndexByte(deIDCardAlphabet, s[i]) < 0 {
			return false
		}
	}
	return icaoCheckDigit(s[:9]) == int(s[9]-'0')
}

// passportWords are fragments of the word passport in the EU languages, in the cases a line is likely to use.
// "pas" covers pass, passport, passeport, pasaport, paspoort, passi, pase and pasas.
var passportWords = []string{"pas", "Pas", "PAS", "паспорт", "Паспорт", "ПАСПОРТ", "διαβατ", "Διαβατ", "ΔΙΑΒΑΤ",
	"putovnic", "Putovnic", "PUTOVNIC", "útlevél", "Útlevél", "ÚTLEVÉL", "utlevel", "Utlevel", "UTLEVEL",
	"potni", "Potni", "POTNI", "ravel doc", "RAVEL DOC"}

// namesPassport is the prefilter: a line that names no passport in any language cannot hold a keyword.
func namesPassport(line string) bool {
	for _, w := range passportWords {
		if strings.Contains(line, w) {
			return true
		}
	}
	return false
}
