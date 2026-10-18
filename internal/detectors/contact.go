package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Contact data has no check digit, so these start at low confidence: reported at the default level only
// when a keyword is nearby. Addresses on documentation and reserved test domains are not reported.
func init() {
	Register(Detector{
		Name:        "email",
		Label:       "Email address",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9-]{1,63}(?:\.[A-Za-z0-9-]{1,63})*\.[A-Za-z]{2,24}`),
		Validate:    validEmail,
		Mask:        maskEmail,
		Keywords:    []string{"email", "e-mail", "mail", "contact"},
		Base:        domain.Low,
		Validation:  "an address outside documentation and test domains",
	})
	Register(Detector{
		Name:        "eu-phone",
		Label:       "Phone number (RO, BG, HU, PL)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\+(?:40|359|36|48)[ .-]?\(?\d{1,3}\)?(?:[ .-]?\d{2,4}){2,4}|\b0(?:7\d{2}|8[789]\d)[ .-]?\d{3}[ .-]?\d{3}\b`),
		Normalize:   normalizePhone,
		Validate:    validPhone,
		Mask:        MaskKeepLast(2),
		Keywords:    []string{"phone", "tel", "telefon", "telephone", "mobile", "mobil", "gsm", "телефон"},
		Base:        domain.Low,
		Validation:  "matches a national numbering plan",
	})
}

var exampleDomains = []string{"example.com", "example.org", "example.net", ".example", ".test", ".invalid", ".localhost", "localhost"}

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

func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '+' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var phonePlans = map[string]*regexp.Regexp{
	"+40":  regexp.MustCompile(`^\+40(?:7\d{8}|[23]\d{8})$`),
	"+359": regexp.MustCompile(`^\+359(?:8[789]\d{7}|2\d{7,8}|[3-9]\d{7,8})$`),
	"+36":  regexp.MustCompile(`^\+36(?:(?:20|30|31|50|70)\d{7}|1\d{7}|[2-9]\d{7})$`),
	"+48":  regexp.MustCompile(`^\+48\d{9}$`),
}

var nationalMobile = regexp.MustCompile(`^0(?:7\d{8}|8[789]\d{7})$`)

func validPhone(s string) bool {
	if strings.HasPrefix(s, "0") {
		return nationalMobile.MatchString(s)
	}
	for prefix, re := range phonePlans {
		if strings.HasPrefix(s, prefix) {
			return re.MatchString(s)
		}
	}
	return false
}
