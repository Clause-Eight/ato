package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// EU driving licence number (field 5 of the EU card licence). Directive 2006/126/EC fixes the card's layout but
// leaves the number to each member state, so the detector keeps a table of national formats. The formats come
// from the public descriptions in Microsoft Purview's sensitive information type reference
// (https://learn.microsoft.com/en-us/purview/sit-defn-<country>-drivers-license-number, read for each country
// below) and are approximate: they describe the usual card format, not every older paper licence.
//
//   - DE: authority code (state letter, two district numbers, one character), five-character serial, check
//     character, issue number. Check: positions 1-9 as base-36 values (A=10 ... Z=35) weighted 9 down to 1, sum mod 11,
//     10 written as X (https://de.wikipedia.org/wiki/F%C3%BChrerscheinnummer).
//   - ES: the holder's DNI or NIE; the letter is the number mod 23 in TRWAGMYFPDXBNJZSQVHLCKE, with X, Y, Z read
//     as 0, 1, 2 (https://es.wikipedia.org/wiki/N%C3%BAmero_de_identificaci%C3%B3n_fiscal).
//   - FI: the holder's personal identity code DDMMYY, century sign, ZZZ, control character from DDMMYYZZZ mod 31
//     (https://en.wikipedia.org/wiki/National_identification_number#Finland).
//   - SE: the holder's personal number YYMMDD-NNNC with a Luhn digit over the nine digits
//     (https://sv.wikipedia.org/wiki/Personnummer_i_Sverige).
//   - CZ E + letter + six digits; EE ET + six digits; HU two letters + six digits; LV three letters + six digits;
//     IE six digits + four letters; IT letter, alphanumeric, seven alphanumerics, letter (an optional space or dash
//     between the groups); MT two characters + six digits; PL 5 digits/2 digits/4 or 7 digits; PT letters, dash,
//     five to eight digits, space, one digit; RO letter + eight digits; SK letter + seven digits.
//
// Not covered: AT, HR, DK, LT (eight digits), BG, GR, SI (nine), BE, NL (ten) and CY, FR (twelve) use a bare
// number with no public check, and the RO, SK and MT formats may also be all digits. A keyword on the line before is
// enough for a keyword-gated detector, so a bare number there cannot be told from an order ID or a timestamp on
// the next line; those forms are left out rather than reported at a precision far below the rest. Luxembourg has
// no published format. The pattern finds runs of capitals, digits and the usual separators, and the national
// formats are checked in Validate, which keeps the scan fast. The prefilter needs six digits, so an Italian number
// with fewer digits is missed. Off by default and only reported with a keyword.
func init() {
	Register(Detector{
		Name:         "driving-licence",
		Label:        "EU driving licence number",
		Country:      domain.CrossBorder,
		Pattern:      regexp.MustCompile(`[A-Z0-9][A-Z0-9/+-]{1,15}(?:(?: - | )[A-Z0-9][A-Z0-9-]{0,7}){0,2}`),
		Normalize:    strings.ToUpper,
		Validate:     validLicence,
		TrimGroups:   true,
		Keywords:     []string{"führerschein", "führerscheinnummer", "permis de conducere", "permis de conduire", "patente di guida", "permiso de conducir", "carné de conducir", "prawo jazdy", "rijbewijs", "rijbewijsnummer", "jogosítvány", "шофьорска книжка", "řidičský průkaz", "carta de condução", "körkort", "ajokortti", "kørekort", "driving licence", "driving license", "driver license", "drivers license", "driver's license", "dl number", "licence number", "license number"},
		Concepts:     []Concept{DrivingLicence},
		Base:         domain.Low,
		Validation:   "matches a member state's licence number format; DE, ES, FI and SE control characters are verified",
		Needs:        Needs{Digits: 6},
		NeedsKeyword: true,
	})
	Register(Detector{
		Name:         "licence-plate",
		Label:        "EU vehicle registration plate",
		Country:      domain.CrossBorder,
		Pattern:      regexp.MustCompile(`[A-ZÄÖÜ0-9]{1,8}(?:[ -][A-Z0-9]{1,5}){0,3}`),
		Normalize:    strings.ToUpper,
		Validate:     validPlate,
		TrimGroups:   true,
		Keywords:     []string{"kennzeichen", "kfz-kennzeichen", "nummernschild", "număr de înmatriculare", "numar inmatriculare", "nr înmatriculare", "plaque d'immatriculation", "immatriculation", "targa", "matrícula", "numer rejestracyjny", "tablica rejestracyjna", "kenteken", "rendszám", "регистрационен номер", "spz", "registreringsnummer", "rekisterinumero", "nummerplade", "licence plate", "license plate", "number plate", "plate number", "registration plate", "vehicle registration", "reg number", "numberplate", "licenseplate", "licenceplate"},
		Concepts:     []Concept{LicencePlate},
		Base:         domain.Low,
		Validation:   "matches a member state's plate format (region codes and letter sets where they are fixed)",
		Needs:        Needs{Digits: 1, Check: hasLetterNextToDigit},
		NeedsKeyword: true,
	})
}

type licenceFormat struct {
	country string
	re      string
	check   func(string) bool
}

var licenceFormats = []licenceFormat{
	{"DE", `[A-Z][0-9]{2}[A-Z0-9]{6}[0-9X][A-Z0-9]`, dlGermanCheck},
	{"ES", `[0-9]{8}[A-Z]|[XYZ][0-9]{7}[A-Z]`, dlSpanishLetter},
	{"FI", `[0-9]{6}[-+A-FU-Y][0-9]{3}[0-9A-Y]`, dlFinnishCheck},
	{"SE", `[0-9]{6}[-+][0-9]{4}`, dlSwedishCheck},
	{"CZ", `E[A-Z] ?[0-9]{6}`, nil},
	{"EE", `ET[0-9]{6}`, nil},
	{"HU", `[A-Z]{2}[0-9]{6}`, nil},
	{"LV", `[A-Z]{3}[0-9]{6}`, nil},
	{"IE", `[0-9]{6}[A-Z]{4}`, nil},
	{"IT", `[A-Z][A-Z0-9](?: |-| - )?[A-Z0-9]{7}(?: |-| - )?[A-Z]`, nil},
	{"MT", `(?:[A-Z]{2}|[A-Z][0-9]|[0-9][A-Z]) ?[0-9]{3} ?[0-9]{3}`, nil},
	{"PL", `[0-9]{5}/[0-9]{2}/[0-9]{4}(?:[0-9]{3})?`, nil},
	{"PT", `[A-Z]{2}-[0-9]{5,6} [0-9]|[A-Z]-[0-9]{6}(?:[0-9]{2})? [0-9]`, nil},
	{"RO", `[A-Z][0-9]{8}`, nil},
	{"SK", `[A-Z][0-9]{7}`, nil},
}

var licenceAnchored = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(licenceFormats))
	for i, f := range licenceFormats {
		out[i] = regexp.MustCompile(`^(?:` + f.re + `)$`)
	}
	return out
}()

func validLicence(s string) bool {
	// Every covered format has a letter or a separator, and none is shorter than 8 or longer than 16 characters.
	if len(s) < 8 || len(s) > 16 || strings.Trim(s, "0123456789") == "" {
		return false
	}
	for i, f := range licenceFormats {
		if licenceAnchored[i].MatchString(s) && (f.check == nil || f.check(s)) {
			return true
		}
	}
	return false
}

func dlBase36(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return -1
}

func dlGermanCheck(s string) bool {
	if len(s) != 11 {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		v := dlBase36(s[i])
		if v < 0 {
			return false
		}
		sum += v * (9 - i)
	}
	want := byte('0' + sum%11)
	if sum%11 == 10 {
		want = 'X'
	}
	return s[9] == want
}

const dlDNILetters = "TRWAGMYFPDXBNJZSQVHLCKE"

func dlSpanishLetter(s string) bool {
	if len(s) != 9 {
		return false
	}
	body := s[:8]
	if i := strings.IndexByte("XYZ", s[0]); i >= 0 {
		body = string(rune('0'+i)) + s[1:8]
	}
	n := 0
	for i := 0; i < len(body); i++ {
		if body[i] < '0' || body[i] > '9' {
			return false
		}
		n = n*10 + int(body[i]-'0')
	}
	return dlDNILetters[n%23] == s[8]
}

const dlHETUControl = "0123456789ABCDEFHJKLMNPRSTUVWXY"

func dlFinnishCheck(s string) bool {
	if len(s) != 11 {
		return false
	}
	d, ok := Digits(s[:6] + s[7:10])
	if !ok {
		return false
	}
	century := 1900
	switch c := s[6]; {
	case c == '+':
		century = 1800
	case c >= 'A' && c <= 'F':
		century = 2000
	}
	if !ValidDate(century+d[4]*10+d[5], d[2]*10+d[3], d[0]*10+d[1]) {
		return false
	}
	if individual := d[6]*100 + d[7]*10 + d[8]; individual < 2 || individual > 899 {
		return false
	}
	n := 0
	for _, v := range d {
		n = n*10 + v
	}
	return dlHETUControl[n%31] == s[10]
}

func dlSwedishCheck(s string) bool {
	if len(s) != 11 {
		return false
	}
	d, ok := Digits(s[:6] + s[7:])
	if !ok {
		return false
	}
	// The century is not written; a valid month and day in any year of the two candidates is enough.
	if !ValidDate(2000+d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]) && !ValidDate(1900+d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]) {
		return false
	}
	return Luhn(s[:6] + s[7:])
}

// Vehicle registration plates of the member states. Sources: the Wikipedia articles "Vehicle registration plates
// of <country>" (https://en.wikipedia.org/wiki/Vehicle_registration_plates_of_the_European_Union and the national
// pages linked from it), read for each format below. Only current car formats are covered, written with the
// usual separators; vanity plates, diplomatic, military and trailer series are not.
//
//   - RO: county code (B, AB, AG ... VS) + two digits (Bucharest two or three) + three letters without Q.
//   - DE: district code of one to three letters, dash or space, one or two letters, space, one to four digits,
//     optional E or H; at most eight letters and digits.
//   - FR (SIV, 2009): AA-123-AA without I, O, U. IT: AA 123 AA without I, O, Q, U. ES: 1234 BBB, consonants only.
//   - PL: two-letter area code + five characters, or three-letter code + four or five, in the published patterns.
//   - NL: sidecodes 4 to 11 (AB-12-CD, AB-CD-12, 12-AB-CD, 12-ABC-3, 1-ABC-23, AB-123-C, A-123-BC, ABC-12-D).
//   - HU: ABC-123 (1990-2022) and AA BB-123 (2022). BG: province code + four digits + two letters from ABEKMHOPCTYX.
//   - CZ: digit, region letter, digit or letter, space, four digits. SK: AA 123 AA. BE: 1-ABC-234.
//   - PT: AA-12-AB, 12-34-AB, 12-AB-34, AB-12-34. SE: ABC 123 / ABC 12A.
//     HR, SI: MM 123-AB. DK: AB 12 345. FI, LT, CY, MT: ABC-123. EE: 123 ABC. LV: AB-1234. IE: 12-D-3456,
//     191-D-12345. GR: ABC-1234. LU: AB 1234.
//
// The pattern only finds groups of capitals and digits; the national formats are checked in Validate, which keeps
// the scan fast. Several short formats (three letters and three digits) are shared by many countries and by product
// codes, so the detector is off by default and only reports a plate next to a keyword.
var plateFormats = []string{
	`(?:B ?[0-9]{2,3}|` + roCounties + ` ?[0-9]{2}) ?[A-PR-Z]{3}`,
	`[A-ZÄÖÜ]{1,3}[- ][A-Z]{1,2} [1-9][0-9]{0,3}[EH]?`,
	`[A-HJ-NP-TV-Z]{2}-[0-9]{3}-[A-HJ-NP-TV-Z]{2}`,
	`[A-HJ-NPR-TV-Z]{2} ?[0-9]{3} ?[A-HJ-NPR-TV-Z]{2}`,
	`[0-9]{4} ?[BCDFGHJKLMNPRSTVWXYZ]{3}`,
	`[A-Z]{2} (?:[0-9]{5}|[0-9]{4}[ACE-HJ-NPR-Y]|[0-9]{3}[ACE-HJ-NPR-Y]{2}|[0-9][ACE-HJ-NPR-Y][0-9]{3}|[0-9][ACE-HJ-NPR-Y]{2}[0-9]{2})`,
	`[A-Z]{3} (?:[ACE-HJ-NPR-Y][0-9]{3}|[0-9]{2}[ACE-HJ-NPR-Y]{2}|[1-9][ACE-HJ-NPR-Y][0-9]{2}|[0-9]{2}[ACE-HJ-NPR-Y][1-9]|[1-9][ACE-HJ-NPR-Y]{2}[1-9]|[ACE-HJ-NPR-Y]{2}[0-9]{2}|[0-9]{5}|[0-9]{4}[ACE-HJ-NPR-Y]|[0-9]{3}[ACE-HJ-NPR-Y]{2})`,
	`[B-DF-HJ-NP-TV-Z]{2}-(?:[0-9]{2}-[B-DF-HJ-NP-TV-Z]{2}|[B-DF-HJ-NP-TV-Z]{2}-[0-9]{2}|[0-9]{3}-[B-DF-HJ-NP-TV-Z])`,
	`[0-9]{2}-(?:[B-DF-HJ-NP-TV-Z]{2}-[B-DF-HJ-NP-TV-Z]{2}|[B-DF-HJ-NP-TV-Z]{3}-[0-9])`,
	`[0-9]-[B-DF-HJ-NP-TV-Z]{3}-[0-9]{2}|[B-DF-HJ-NP-TV-Z]-[0-9]{3}-[B-DF-HJ-NP-TV-Z]{2}|[B-DF-HJ-NP-TV-Z]{3}-[0-9]{2}-[B-DF-HJ-NP-TV-Z]`,
	`[A-Z]{3}-[0-9]{3}|[A-Z]{2} [A-Z]{2}-[0-9]{3}`,
	`(?:` + bgProvinces + `) ?[0-9]{4} ?[ABEKMHOPCTX]{2}`,
	`[1-9][ABCEHJKLMPSTUZ][0-9A-Z] ?[0-9]{4}`,
	`[A-Z]{2} ?[0-9]{3} ?[A-Z]{2}`,
	`[1-9]-[A-Z]{3}-[0-9]{3}`,
	`[A-Z]{2}-[0-9]{2}-[A-Z]{2}|[0-9]{2}-[0-9]{2}-[A-Z]{2}|[0-9]{2}-[A-Z]{2}-[0-9]{2}|[A-Z]{2}-[0-9]{2}-[0-9]{2}`,
	`[A-HJ-PR-UW-Z]{3} ?[0-9]{2}[0-9A-HJ-PR-UW-Z]`,
	`[A-Z]{2} [0-9]{3,4}-[A-Z]{1,2}`,
	`[A-Z]{2} ?[0-9]{2} ?[0-9]{3}`,
	`[0-9]{3} ?[A-Z]{3}`,
	`[A-Z]{2}-[0-9]{4}`,
	`[0-9]{2,3}-(?:` + ieCounties + `)-[0-9]{1,6}`,
	`[A-Z]{3}-[0-9]{4}`,
	`[A-Z]{2} [0-9]{4}`,
}

const roCounties = `(?:AB|AG|AR|BC|BH|BN|BR|BT|BV|BZ|CJ|CL|CS|CT|CV|DB|DJ|GJ|GL|GR|HD|HR|IF|IL|IS|MH|MM|MS|NT|OT|PH|SB|SJ|SM|SV|TL|TM|TR|VL|VN|VS)`

const bgProvinces = `A|B|BH|BP|BT|E|EB|EH|K|KH|M|H|OB|P|PA|PB|PK|PP|C|CA|CB|CH|CM|CO|CC|CT|T|TX|Y|X`

const ieCounties = `C|CE|CN|CW|D|DL|G|KE|KK|KY|L|LD|LH|LK|LM|LS|MH|MN|MO|OY|RN|SO|T|W|WH|WX|WW`

var plateAnchored = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(plateFormats))
	for i, f := range plateFormats {
		out[i] = regexp.MustCompile(`^(?:` + f + `)$`)
	}
	return out
}()

func validPlate(s string) bool {
	// No format has more than three letters or five digits in a row, and none has more than ten of them in all.
	letters, digits, letterRun, digitRun := 0, 0, 0, 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
			digitRun++
			letterRun = 0
		case r == ' ' || r == '-':
			letterRun, digitRun = 0, 0
		default:
			letters++
			letterRun++
			digitRun = 0
		}
		if letterRun > 3 || digitRun > 5 {
			return false
		}
	}
	if letters == 0 || digits == 0 || letters+digits > 10 {
		return false
	}
	for _, re := range plateAnchored {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// hasLetterNextToDigit is the plate prefilter: every plate has a run of one to three capital letters next to a
// digit, directly or across one space or dash.
func hasLetterNextToDigit(line string) bool {
	isUpper := func(b byte) bool { return b >= 'A' && b <= 'Z' }
	isLetter := func(b byte) bool { return isUpper(b) || b >= 'a' && b <= 'z' || b >= 0x80 }
	isDigit := func(b byte) bool { return b >= '0' && b <= '9' }
	digitAt := func(k, step int) bool {
		if k >= 0 && k < len(line) && (line[k] == ' ' || line[k] == '-') {
			k += step
		}
		return k >= 0 && k < len(line) && isDigit(line[k])
	}
	for i := 0; i < len(line); i++ {
		if !isUpper(line[i]) || i > 0 && isLetter(line[i-1]) && line[i-1] < 0x80 {
			continue
		}
		j := i
		for j < len(line) && isUpper(line[j]) {
			j++
		}
		if j-i <= 3 && (j == len(line) || !isLetter(line[j])) && (digitAt(i-1, -1) || digitAt(j, 1)) {
			return true
		}
		i = j
	}
	return false
}
