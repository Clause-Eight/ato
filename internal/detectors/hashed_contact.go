package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Hashed email addresses and phone numbers sent to advertising platforms for matching: Meta's Conversions API
// (user_data with em, ph and external_id, SHA-256 hex,
// https://developers.facebook.com/docs/marketing-api/conversions-api/parameters/customer-information-parameters),
// Google Ads enhanced conversions (sha256_email_address, sha256_phone_number, hashedEmail, hashedPhoneNumber,
// https://developers.google.com/google-ads/api/docs/conversions/enhanced-conversions/overview) and TikTok's
// Events API (user with email and phone, SHA-256 hex). A hash of an email or phone number is pseudonymous data,
// which is still personal data (GDPR Recital 26): the platform matches it to a person. A 64- or 32-digit hex
// value is anything (a commit, a checksum), so it is reported only under one of these keys; the short Meta keys
// em, ph and external_id only inside a user_data object or next to another of its keys. The hashes of the empty
// string are left out as sample data.
func init() {
	Register(Detector{
		Name:        "hashed-contact",
		Label:       "Hashed email or phone number (ad matching)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[0-9A-Fa-f]{64}|[0-9A-Fa-f]{32}`),
		Normalize:   strings.ToLower,
		Validate:    validDigest,
		Keywords:    []string{"hashed_email", "email_sha256", "sha256_email", "sha256_email_address", "hashed_phone", "hashed_phone_number", "sha256_phone_number", "user_data", "userdata", "hashedemail", "hashedphonenumber", "em", "ph", "external_id"},
		Base:        domain.Medium,
		Samples: map[string]bool{
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855": true, // SHA-256 of ""
			"d41d8cd98f00b204e9800998ecf8427e":                                 true, // MD5 of ""
		},
		Validation: "a SHA-256 or MD5 hex digest under a hashed email or phone key",
		Needs:      Needs{Check: maybeHashedContact},
		Context:    hashedContactContext,
	})
}

// hashedContactKeys name a hashed email or phone number on their own, folded and written together. The plain
// email and phone keys count too: a hex digest under them is a hashed address (TikTok's user object).
var hashedContactKeys = words("hashedemail", "hashedemailaddress", "emailsha256", "sha256email", "sha256emailaddress",
	"emailhash", "emailhashed", "hashedmail", "md5email", "emailmd5", "hashedphone", "hashedphonenumber", "phonesha256",
	"sha256phone", "sha256phonenumber", "phonehash", "phonehashed", "phonenumbersha256", "md5phone", "phonemd5",
	"email", "emailaddress", "phone", "phonenumber", "hem")

// adMatchKeys are the short keys of Meta's user_data that hold a hashed email, phone number or customer ID.
var adMatchKeys = words("em", "ph", "externalid")

// userDataKey finds a user_data object (or userData, or Meta's short-key neighbours on one line) above a value.
var userDataKey = regexp.MustCompile(`(?i)\buser_?data\b`)

// adMatchNeighbour is another short user_data key on the same line, as in {"em": [...], "ph": [...]}.
var adMatchNeighbour = regexp.MustCompile(`["'\s{,](?:em|ph|fn|ln|ge|db|ct|st|zp|country|external_id|client_ip_address|fbp|fbc)["']?\s*:`)

// maybeHashedContact reports whether a line has a run of 32 hex digits.
func maybeHashedContact(line string) bool {
	run := 0
	for i := 0; i < len(line); i++ {
		if !isHex(line[i]) {
			run = 0
			continue
		}
		if run++; run == 32 {
			return true
		}
	}
	return false
}

func validDigest(s string) bool {
	if len(s) != 64 && len(s) != 32 {
		return false
	}
	return strings.Trim(s, s[:1]) != ""
}

func hashedContactContext(a Around) Fit {
	before := a.Before
	if a.Column == "" && strings.Trim(before, " \t\"'`") == "" {
		// An array item on a line of its own: the key is on a line above.
		before = a.Record[:strings.LastIndexByte(a.Record, '\n')+1] + before
	}
	key := keyTokens(trimDigests(before))
	if labelled(key, hashedContactKeys) || columnLabel(a, hashedContactKeys) {
		return Fits
	}
	if !labelled(key, adMatchKeys) && !columnLabel(a, adMatchKeys) {
		return Unfit
	}
	if userDataKey.MatchString(a.Column) || userDataKey.MatchString(a.Record) {
		return Fits
	}
	line := a.Record[strings.LastIndexByte(a.Record, '\n')+1:]
	if len(adMatchNeighbour.FindAllStringIndex(line, 2)) >= 2 {
		return Fits
	}
	return Unfit
}

// trimDigests drops the digests listed before a value in the same array, so that the second hash of
// "em": ["…", "…"] still sees its key.
func trimDigests(before string) string {
	for {
		s := strings.TrimRight(before, " \t\r\n\"'`")
		if !strings.HasSuffix(s, ",") {
			return before
		}
		s = strings.TrimRight(strings.TrimSuffix(s, ","), " \t\r\n\"'`")
		i := len(s)
		for i > 0 && isHex(s[i-1]) {
			i--
		}
		if n := len(s) - i; n != 32 && n != 64 {
			return before
		}
		before = s[:i]
	}
}
