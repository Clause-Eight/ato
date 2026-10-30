package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// IMEI (3GPP TS 23.003 section 6.2.1): 15 digits, written AA-BBBBBB-CCCCCC-D. The first eight are the type
// allocation code, whose first two are the reporting body identifier; the last is a Luhn check digit over the
// other fourteen. Only reporting bodies allocated by the GSMA (TS.06 Annex A, current and retired) are accepted;
// test ranges 00-09 are not, and neither is anything that is also a valid payment card number, such as a
// fifteen-digit Amex number. Sources: https://www.3gpp.org/ftp/Specs/archive/23_series/23.003/ and
// https://en.wikipedia.org/wiki/Reporting_Body_Identifier.
//
// Mobile advertising IDs: Apple's IDFA and Google's advertising ID are UUIDs (RFC 9562 text form) that the
// user can reset. An app that may not track gets the all-zero UUID, which identifies nobody
// (https://developer.apple.com/documentation/adsupport/asidentifiermanager/advertisingidentifier,
// https://support.google.com/googleplay/android-developer/answer/6048248). A UUID is anything in code, so the
// keyword is required.
func init() {
	Register(Detector{
		Name:        "imei",
		Label:       "Mobile device identity (IMEI)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{2}[ -]?\d{6}[ -]?\d{6}[ -]?\d`),
		Normalize:   DigitsOnly,
		Validate:    validIMEI,
		Keywords:    []string{"imei", "imeisv"},
		Base:        domain.Low,
		Formatted:   regexp.MustCompile(`^\d{2}-\d{6}-\d{6}-\d$|^\d{2} \d{6} \d{6} \d$`),
		Validation:  "reporting body and Luhn check digit are valid",
	})
	Register(Detector{
		Name:         "advertising-id",
		Label:        "Mobile advertising ID (IDFA, GAID)",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}`),
		Normalize:    strings.ToLower,
		Validate:     validAdvertisingID,
		Keywords:     []string{"idfa", "gaid", "advertising id", "advertisingid", "advertising identifier", "advertisingidentifier", "adid", "aaid", "ifa"},
		Base:         domain.Low,
		Validation:   "a UUID that is not the all-zero opt-out value",
		NeedsKeyword: true,
		Needs:        Needs{Check: maybeUUID},
	})
}

// imeiReportingBodies are the two-digit reporting body identifiers that allocate (or allocated) type codes.
var imeiReportingBodies = map[string]bool{
	"01": true, "10": true, "30": true, "33": true, "35": true, "44": true, "45": true, "49": true, "50": true,
	"51": true, "52": true, "53": true, "54": true, "86": true, "91": true, "99": true,
}

func validIMEI(s string) bool {
	if len(s) != 15 || !imeiReportingBodies[s[:2]] || !Luhn(s) {
		return false
	}
	return CardBrand(s) == ""
}

func validAdvertisingID(s string) bool {
	if len(s) != 36 || strings.Count(s, "-") != 4 {
		return false
	}
	hex := strings.ReplaceAll(s, "-", "")
	if len(hex) != 32 || strings.Trim(hex, "0") == "" || strings.Trim(hex, "f") == "" {
		return false
	}
	for i := 0; i < len(hex); i++ {
		if !isHex(hex[i]) {
			return false
		}
	}
	return true
}

// maybeUUID looks for the 8-4 hex groups that start a UUID.
func maybeUUID(line string) bool {
	for i := 8; i+5 < len(line); i++ {
		if line[i] == '-' && line[i+5] == '-' && isHex(line[i-1]) && isHex(line[i+1]) {
			return true
		}
	}
	return false
}
