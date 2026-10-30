package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Contact data has no check digit, so these start at low confidence: reported at the default level only
// when a keyword is nearby. Addresses on documentation and reserved test domains are not reported.
//
// Phone numbers in the 27 member states: an international number (+CC or 00CC, then the national significant
// number with optional spaces, dots, dashes, brackets and a "(0)" trunk prefix) must fit its country's plan,
// by leading digits and length of mobile and geographic numbers. Written with a +, it is reported without a
// keyword. Plans: the national numbering plans notified to ITU-T under Recommendation E.129
// (https://www.itu.int/oth/T0202.aspx?parent=T0202), cross-checked with the per-country summaries at
// https://en.wikipedia.org/wiki/Telephone_numbers_in_Europe. Germany, Austria, Finland and Sweden have open
// plans with variable lengths, so only their ranges are checked. Of the national formats only the Romanian
// and Bulgarian mobile ones (07xx xxx xxx, 087-089) are kept: elsewhere a national number is a 6-10 digit run.
func init() {
	Register(Detector{
		Name:        "email",
		Label:       "Email address",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9-]{1,63}(?:\.[A-Za-z0-9-]{1,63})*\.[A-Za-z]{2,24}`),
		Validate:    validEmail,
		Mask:        maskEmail,
		Needs:       Needs{AnyOf: []string{"@"}},
		Keywords:    []string{"email", "e-mail", "mail", "contact"},
		Concepts:    []Concept{Email},
		Base:        domain.Low,
		Validation:  "an address outside documentation and test domains",
	})
	Register(Detector{
		Name:        "eu-phone",
		Label:       "Phone number",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:\+|\b00)` + euCallingCodes + `(?:[ .-]?\(0\))?(?:[ .-]?\(?\d{1,4}\)?){2,7}|\b0(?:7\d{2}|8[789]\d)[ .-]?\d{3}[ .-]?\d{3}\b`),
		Normalize:   normalizePhone,
		Validate:    validPhone,
		Mask:        MaskKeepLast(2),
		Keywords:    []string{"phone", "tel", "telefon", "telephone", "mobile", "mobil", "gsm", "телефон"},
		Concepts:    []Concept{Phone},
		Base:        domain.Low,
		Validation:  "matches the country's national numbering plan",
		TrimGroups:  true,
		Formatted:   regexp.MustCompile(`^\+`),
		Needs:       Needs{AnyOf: []string{"+3", "+4", "00", "07", "08"}},
	})
}

// exampleDomains are reserved for documentation and tests (RFC 2606, RFC 6761), and the placeholder domains that
// test data uses the same way. An address on them is not reported.
var exampleDomains = []string{"example.com", "example.org", "example.net", ".example", ".test", ".invalid", ".localhost", "localhost",
	"test.com", "email.com", "domain.com", "yourdomain.com", "mydomain.com", "yourcompany.com", "company.com", "abc.com", "xyz.com",
	"foo.com", "bar.com", "baz.com", "foobar.com", "acme.com", "sample.com", "somewhere.com", "someone.com"}

func validEmail(s string) bool {
	at := strings.LastIndexByte(s, '@')
	if at <= 0 || strings.Contains(s, "..") {
		return false
	}
	domainPart := strings.ToLower(s[at+1:])
	for _, ex := range exampleDomains {
		if domainPart == strings.TrimPrefix(ex, ".") || strings.HasSuffix(domainPart, ex) {
			return false
		}
	}
	return true
}

func maskEmail(s string) string {
	at := strings.LastIndexByte(s, '@')
	if at < 0 {
		return MaskAll(s)
	}
	return MaskAll(s[:at]) + s[at:]
}

// euCallingCodes are the E.164 country codes of the 27 member states. Country codes are prefix-free, so a
// number has at most one reading.
const euCallingCodes = `(?:3[0-469]|35[1-36-9]|37[0-2]|38[56]|42[01]|4[035689])`

// phonePlans hold the national significant number (after the country code, without the trunk 0) of mobile and
// geographic numbers in each member state: leading digits and lengths, not every allocated block.
var phonePlans = map[string]*regexp.Regexp{
	"43":  regexp.MustCompile(`^(?:1\d{5,12}|[2-7]\d{6,12})$`),                         // AT, open plan
	"32":  regexp.MustCompile(`^(?:4[5-9]\d{7}|[1-9]\d{7})$`),                          // BE
	"359": regexp.MustCompile(`^(?:(?:8[7-9]|98)\d{7}|2\d{6,7}|[3-9]\d{6,7})$`),        // BG
	"385": regexp.MustCompile(`^(?:9[1-9]\d{6,7}|[1-5]\d{6,7})$`),                      // HR
	"357": regexp.MustCompile(`^(?:2[2-6]\d{6}|9[4-9]\d{6})$`),                         // CY
	"420": regexp.MustCompile(`^[2-7]\d{8}$`),                                          // CZ
	"45":  regexp.MustCompile(`^[2-9]\d{7}$`),                                          // DK
	"372": regexp.MustCompile(`^(?:5\d{6,7}|8[1-7]\d{6}|[3467]\d{6})$`),                // EE
	"358": regexp.MustCompile(`^(?:(?:4\d|50)\d{7,8}|1[3-9]\d{4,8}|[235689]\d{5,9})$`), // FI, open plan
	"33":  regexp.MustCompile(`^[1-79]\d{8}$`),                                         // FR
	"49":  regexp.MustCompile(`^(?:1[5-7]\d{8,9}|[2-9]\d{5,10})$`),                     // DE, open plan
	"30":  regexp.MustCompile(`^(?:2\d{9}|69\d{8})$`),                                  // GR
	"36":  regexp.MustCompile(`^(?:(?:20|30|31|50|70)\d{7}|1\d{7}|[2-9]\d{7})$`),       // HU
	"353": regexp.MustCompile(`^(?:8[3-9]\d{7}|1\d{7}|[2-9]\d{6,8})$`),                 // IE
	"39":  regexp.MustCompile(`^(?:0\d{5,10}|3\d{8,9})$`),                              // IT, keeps the 0
	"371": regexp.MustCompile(`^[26]\d{7}$`),                                           // LV
	"370": regexp.MustCompile(`^[3-6]\d{7}$`),                                          // LT
	"352": regexp.MustCompile(`^(?:6[2-9]1\d{6}|2\d{5,10}|[3-9]\d{5})$`),               // LU
	"356": regexp.MustCompile(`^[25789]\d{7}$`),                                        // MT
	"31":  regexp.MustCompile(`^(?:6[1-8]\d{7}|[1-57]\d{8})$`),                         // NL
	"48":  regexp.MustCompile(`^[1-9]\d{8}$`),                                          // PL
	"351": regexp.MustCompile(`^(?:2\d{8}|9[1236]\d{7})$`),                             // PT
	"40":  regexp.MustCompile(`^(?:7\d{8}|[23]\d{8})$`),                                // RO
	"421": regexp.MustCompile(`^[2-59]\d{8}$`),                                         // SK
	"386": regexp.MustCompile(`^[1-7]\d{7}$`),                                          // SI
	"34":  regexp.MustCompile(`^[6-9]\d{8}$`),                                          // ES
	"46":  regexp.MustCompile(`^(?:7[02369]\d{7}|[1-689]\d{6,8})$`),                    // SE, open plan
}

// nationalMobile is the national form kept without a country code: Romanian 07 and Bulgarian 087-089 mobiles.
// Other national formats are short digit runs that look like any other number.
var nationalMobile = regexp.MustCompile(`^0(?:7\d{8}|8[789]\d{7})$`)

// normalizePhone keeps the digits and a leading +, turns a 00 international prefix into +, and drops the
// "(0)" trunk prefix that is often written after the country code.
func normalizePhone(s string) string {
	s = strings.Replace(s, "(0)", "", 1)
	var b strings.Builder
	for _, r := range s {
		if r == '+' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if strings.HasPrefix(out, "00") {
		out = "+" + out[2:]
	}
	return out
}

func validPhone(s string) bool {
	if strings.HasPrefix(s, "+") {
		digits := s[1:]
		for n := 2; n <= 3 && n < len(digits); n++ {
			if re, ok := phonePlans[digits[:n]]; ok {
				return re.MatchString(digits[n:])
			}
		}
		return false
	}
	return nationalMobile.MatchString(s)
}
