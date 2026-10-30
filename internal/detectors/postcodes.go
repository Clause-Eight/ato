package detectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Postcodes of the 27 member states. A postcode alone is weak personal data and has no check digit, so the
// detector is off by default and needs a postcode word nearby (not just "address", which in code is usually a
// memory or network address). Formats, from the Universal
// Postal Union's addressing pages (https://www.upu.int/en/Postal-Solutions/Programmes-Services/Addressing-Solutions)
// and the national operators, as summarised at https://en.wikipedia.org/wiki/List_of_postal_codes:
//
//	4 digits         AT, BE, BG, CY, DK (also 0800-0999), HU, LU (L-1234), SI (SI-1000), LV (LV-1234)
//	5 digits         DE, EE, ES (province 01-52), FI, FR, HR (first digit 1-5), IT, LT (LT-12345)
//	NNN NN           CZ (first digit 1-7), SK (0, 8, 9), GR (1-8), SE (1-9)
//	6 digits         RO
//	NN-NNN           PL
//	NNNN-NNN         PT
//	NNNN AA          NL, without the letter pairs SA, SD and SS (PostNL)
//	A65 F4E2         IE Eircode: routing key letter-digit-digit (or D6W) and four characters from 0-9 and
//	                 ACDEFHKNPRTVWXY (https://www.eircode.ie/what-is-eircode)
//	ABC 1234         MT: a locality prefix from MaltaPost's list and four digits
//
// A two-letter ISO prefix (DE-10115) or the older one-letter CEPT prefix (D-10115) is accepted when the
// digits fit that country.
func init() {
	Register(Detector{
		Name:         "postcode",
		Label:        "Postcode (EU member states)",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      postcodePattern,
		Normalize:    normalizePostcode,
		Validate:     validPostcode,
		Keywords:     postcodeWords,
		Base:         domain.Low,
		Validation:   "matches a member state's postcode format",
		NeedsKeyword: true,
		Needs:        Needs{Check: digitsOrEircode},
	})
}

// postcodeWords are the postcode concept's words in every language without the bare "zip", which in code
// is far more often an archive or a function than a postcode.
var postcodeWords = []string{
	"postcode", "postal_code", "zip_code", "zipcode", "plz", "eircode", "post code", "postal code", "zip code",
	"postleitzahl", "code postal", "codice postale", "código postal", "cod poștal", "kod pocztowy", "psč",
	"poštovní směrovací číslo", "poštové smerovacie číslo", "irányítószám", "пощенски код", "ταχυδρομικός κώδικας",
	"τκ", "postnummer", "postinumero", "postiindeks", "sihtnumber", "pasta indekss", "pašto kodas",
	"poštanski broj", "poštna številka", "kodiċi postali", "éirchód", "cód poist",
}

const eircodeChars = "0-9ACDEFHKNPRTVWXY"

// A bare number is a postcode candidate only where a postcode stands: at the end of a value (a closing quote, a
// tag or the end of the line) or before a place name ("10115 Berlin"). Runs of digit groups such as dates and
// account numbers are matched whole so that no postcode is read out of them, and then rejected.
var postcodePattern = regexp.MustCompile(
	`(?:[A-Z]{1,2}-)?\d{4,6}(?:["'<]|$|,? \p{Lu}\p{L}+(?:[ -]\p{Lu}\p{L}+)?)` +
		`|[A-Z]{1,2}-\d{4,6}` +
		`|\d{2}-\d{3}|\d{4}-\d{3}|\d{3} \d{2}|\d{4} ?[A-Z]{2}` +
		`|(?:[ACDEFHKNPRTVWXY]\d{2}|D6W) ?[` + eircodeChars + `]{4}` +
		`|[A-Z]{3} ?\d{4}` +
		`|\d+(?:(?:[./-] ?| )\d+)+\.?["']?`)

var (
	postcodeDutchStart = regexp.MustCompile(`^[1-9]\d{3} ?[A-Z]{2}`)
	postcodeBareStart  = regexp.MustCompile(`^(?:[A-Z]{1,2}-)?\d{4,6}`)
)

// normalizePostcode drops a closing quote or tag and a following place name, keeping the postcode.
func normalizePostcode(s string) string {
	s = strings.TrimRight(s, `"'<`)
	if m := postcodeDutchStart.FindString(s); m != "" && (len(m) == len(s) || s[len(m)] == ' ') {
		return m
	}
	if m := postcodeBareStart.FindString(s); m != "" && len(m) < len(s) {
		rest := strings.TrimLeft(s[len(m):], ", ")
		if rest != "" && rest[0] != '-' && (rest[0] < '0' || rest[0] > '9') && len(rest) < len(s[len(m):]) {
			return m
		}
	}
	return s
}

var (
	digitPostcode = regexp.MustCompile(`^(?:([A-Z]{1,2})-)?(\d{4,6})$`)
	spacedFive    = regexp.MustCompile(`^\d{3} \d{2}$`)
	polish        = regexp.MustCompile(`^\d{2}-\d{3}$`)
	portuguese    = regexp.MustCompile(`^[1-9]\d{3}-\d{3}$`)
	dutch         = regexp.MustCompile(`^[1-9]\d{3} ?([A-Z]{2})$`)
	eircode       = regexp.MustCompile(`^(?:[ACDEFHKNPRTVWXY]\d{2}|D6W) ?[` + eircodeChars + `]{4}$`)
	maltese       = regexp.MustCompile(`^([A-Z]{3}) ?\d{4}$`)
)

// postcodePrefixDigits is the digit count each country prefix takes (ISO 3166 and the older CEPT letters).
var postcodePrefixDigits = map[string]int{
	"A": 4, "AT": 4, "B": 4, "BE": 4, "BG": 4, "CY": 4, "DK": 4, "H": 4, "HU": 4, "L": 4, "LU": 4, "SI": 4, "LV": 4,
	"D": 5, "DE": 5, "EE": 5, "E": 5, "ES": 5, "FI": 5, "F": 5, "FR": 5, "HR": 5, "I": 5, "IT": 5, "LT": 5,
	"CZ": 5, "SK": 5, "GR": 5, "S": 5, "SE": 5,
	"RO": 6,
}

// maltaLocalities are MaltaPost's locality prefixes.
var maltaLocalities = map[string]bool{}

func init() {
	for _, c := range strings.Fields(`ATD BBG BKR BML BRG BZN CBD DGL FGR FNT FRN GDJ GHR GRB GSM KMN GSR GXQ GZR HMR IKL
		ISL KCM KKP KKR SCM LJA LQA MDN MGR MLH MQB MRS MTP MSD MSK MST MTF MXK MXR NDR NXR PBK PLA PTA QLA QRD QRM
		RBT SFI SGN SGW SLC SLM SLZ SNT SPB STJ SPK SVR SWQ TXN VCT VLT XBX XJR XLN XRA XWK ZBB MFN ZBG ZBR ZRQ ZTN`) {
		maltaLocalities[c] = true
	}
}

func validPostcode(s string) bool {
	if m := digitPostcode.FindStringSubmatch(s); m != nil {
		return validDigitPostcode(m[1], m[2])
	}
	switch {
	case spacedFive.MatchString(s):
		return s != "000 00"
	case polish.MatchString(s):
		return s != "00-000"
	case portuguese.MatchString(s):
		return true
	case eircode.MatchString(s):
		return true
	}
	if m := dutch.FindStringSubmatch(s); m != nil {
		return m[1] != "SA" && m[1] != "SD" && m[1] != "SS"
	}
	if m := maltese.FindStringSubmatch(s); m != nil {
		return maltaLocalities[m[1]]
	}
	return false
}

func validDigitPostcode(prefix, d string) bool {
	if prefix != "" {
		n, ok := postcodePrefixDigits[prefix]
		if !ok || n != len(d) {
			return false
		}
	}
	v, _ := strconv.Atoi(d)
	switch len(d) {
	case 4:
		return v >= 1000 || v >= 800 && (prefix == "" || prefix == "DK")
	case 5:
		switch prefix {
		case "D", "DE":
			return v >= 1001
		case "E", "ES":
			return v/1000 >= 1 && v/1000 <= 52
		case "HR":
			return d[0] >= '1' && d[0] <= '5'
		case "CZ":
			return d[0] >= '1' && d[0] <= '7'
		case "SK":
			return d[0] == '0' || d[0] == '8' || d[0] == '9'
		case "GR":
			return d[0] >= '1' && d[0] <= '8'
		case "S", "SE", "EE":
			return d[0] != '0'
		}
		return v > 0
	case 6:
		return d[:2] != "00"
	}
	return false
}

// digitsOrEircode is the postcode prefilter: a run of four to six digits on its own, two to four digits
// followed by a dash or a space and another digit (PL, PT, CZ), or an Eircode routing key.
func digitsOrEircode(line string) bool {
	run := 0
	for i := 0; i <= len(line); i++ {
		if i < len(line) && isASCIIDigit(line[i]) {
			run++
			continue
		}
		if run >= 4 && run <= 6 || run >= 2 && run <= 4 && i+1 < len(line) && (line[i] == '-' || line[i] == ' ') && isASCIIDigit(line[i+1]) {
			return true
		}
		run = 0
		if i+2 < len(line) && strings.IndexByte("ACDEFHKNPRTVWXY", line[i]) >= 0 && isASCIIDigit(line[i+1]) && (isASCIIDigit(line[i+2]) || line[i+2] == 'W') {
			return true
		}
	}
	return false
}
