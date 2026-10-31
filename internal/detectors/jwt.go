package detectors

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"sync"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// JSON Web Tokens that carry personal data. A JWT (RFC 7519) is three base64url segments joined by dots; the
// second is a JSON object of claims, readable by anyone who holds the token. It is reported when a claim is one
// of the OpenID Connect standard claims about a person (email, name, given_name, family_name, middle_name,
// phone_number, birthdate, address; OpenID Connect Core 1.0 section 5.1,
// https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims) with a real value, or when another
// detector confirms a claim's value on its own (an IBAN, a national number). Placeholder names such as
// "John Doe" and addresses on documentation domains do not count. Claim values never reach a report: the mask
// keeps only the header segment, which names the algorithm.
func init() {
	Register(Detector{
		Name:        "jwt-personal",
		Label:       "JWT with personal claims",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]*`),
		Validate:    personalJWT,
		Mask:        maskJWT,
		Keywords:    []string{"jwt", "id_token", "idtoken", "access_token", "accesstoken", "bearer"},
		Base:        domain.Medium,
		Validation:  "the decoded payload holds a personal claim, or a claim value another detector confirms",
		Needs:       Needs{AnyOf: []string{"eyJ"}},
	})
}

// jwtClaims reads a token's payload. ok is false when it is not base64url JSON.
func jwtClaims(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return nil, false
	}
	return claims, true
}

func personalJWT(token string) bool {
	claims, ok := jwtClaims(token)
	if !ok {
		return false
	}
	for k, v := range claims {
		if personalClaim(strings.ToLower(k), v) {
			return true
		}
	}
	return confirmedValue(claims, 0)
}

// placeholderNames are the names documentation and test tokens use.
var placeholderNames = words("john doe", "jane doe", "john smith", "jane smith", "max mustermann", "erika mustermann",
	"test user", "test", "user", "admin", "example", "foo", "bar", "john", "jane", "doe", "smith", "alice", "bob",
	"max", "erika", "mustermann", "string", "name", "first", "last", "null", "undefined")

func personalClaim(key string, v any) bool {
	s, isString := v.(string)
	s = strings.TrimSpace(s)
	switch key {
	case "email":
		return isString && strings.Contains(s, "@") && validEmail(s)
	case "name":
		return isString && strings.Contains(s, " ") && realName(s)
	case "given_name", "family_name", "middle_name":
		return isString && realName(s)
	case "phone_number":
		return isString && len(DigitsOnly(s)) >= 6
	case "birthdate":
		return isString && len(DigitsOnly(s)) >= 4
	case "address":
		if m, ok := v.(map[string]any); ok {
			return len(m) > 0
		}
		return isString && len(s) > 3
	}
	return false
}

func realName(s string) bool {
	if len(s) < 2 || placeholderNames[strings.ToLower(s)] {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' || r == '{' || r == '$' || r == '<' {
			return false
		}
	}
	return true
}

var (
	claimDetectorsOnce sync.Once
	claimDetectors     []Detector
)

// confirmedValue reports whether a string claim, at any depth, is wholly a value that an on-by-default detector
// with its own check confirms, or an email address or phone number under any claim name.
func confirmedValue(v any, depth int) bool {
	if depth > 4 {
		return false
	}
	switch t := v.(type) {
	case map[string]any:
		for _, x := range t {
			if confirmedValue(x, depth+1) {
				return true
			}
		}
	case []any:
		for _, x := range t {
			if confirmedValue(x, depth+1) {
				return true
			}
		}
	case string:
		claimDetectorsOnce.Do(func() {
			for _, d := range All() {
				if d.OnByDefault && d.Validate != nil && !d.NeedsKeyword && d.Context == nil && (d.Base >= domain.Medium || d.Name == "email" || d.Name == "eu-phone") {
					claimDetectors = append(claimDetectors, d)
				}
			}
		})
		s := strings.TrimSpace(t)
		if len(s) < 6 || len(s) > 64 {
			return false
		}
		for _, d := range claimDetectors {
			if d.Pattern.FindString(s) != s {
				continue
			}
			if n := d.Normalize(s); d.Validate(n) && !d.Samples[n] {
				return true
			}
		}
	}
	return false
}

// maskJWT keeps the header segment and hides the claims and the signature.
func maskJWT(v string) string {
	i := strings.IndexByte(v, '.')
	if i < 0 {
		return MaskAll(v)
	}
	return v[:i+1] + MaskAll(v[i+1:])
}
