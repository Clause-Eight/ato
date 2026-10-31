package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// International mobile subscriber identity (IMSI, 3GPP TS 23.003 section 2.2): fifteen digits stored on the SIM,
// the mobile country code (three digits), the network code (two or three) and the subscriber number. It
// identifies the subscriber, not the device. Only the country codes of the EU, the rest of the EEA, Switzerland
// and the United Kingdom are accepted; there is no control digit, so a keyword is required and it is off by
// default. Sources: https://www.3gpp.org/ftp/Specs/archive/23_series/23.003/, ITU-T E.212 and its list of mobile
// country codes as kept in python-stdnum's imsi.dat, https://github.com/arthurdejong/python-stdnum/blob/master/stdnum/imsi.dat
func init() {
	Register(Detector{
		Name:         "imsi",
		Label:        "Mobile subscriber identity (IMSI)",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`2\d{14}`),
		Validate:     validIMSI,
		NeedsKeyword: true,
		Keywords:     []string{"imsi", "international mobile subscriber identity", "subscriber identity"},
		Base:         domain.Low,
		Validation:   "fifteen digits with a European mobile country code; there is no control digit",
	})
}

// imsiCountries are the mobile country codes (ITU-T E.212) of the countries ATO covers.
var imsiCountries = map[string]bool{
	"202": true, "204": true, "206": true, "208": true, "214": true, "216": true, "219": true, "222": true, // GR NL BE FR ES HU HR IT
	"226": true, "228": true, "230": true, "231": true, "232": true, "234": true, "235": true, "238": true, // RO CH CZ SK AT GB GB DK
	"240": true, "242": true, "244": true, "246": true, "247": true, "248": true, "260": true, "262": true, // SE NO FI LT LV EE PL DE
	"268": true, "270": true, "272": true, "274": true, "278": true, "280": true, "284": true, "293": true, // PT LU IE IS MT CY BG SI
	"295": true, // LI
}

func validIMSI(s string) bool {
	_, ok := Digits(s)
	return ok && len(s) == 15 && imsiCountries[s[:3]]
}
