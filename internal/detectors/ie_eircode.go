package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Eircode, the Irish postcode, is given to each address rather than to a street or an area, so on its own it
// points at one home. It is a three-character routing key and a four-character unique identifier drawn from
// 0-9 and the letters ACDEFHKNPRTVWXY, usually written with a space between them (A65 F4E2)
// (https://www.eircode.ie/what-is-eircode). There is no check character; the validation is the fixed list of
// 139 routing keys, from https://en.wikipedia.org/wiki/List_of_Eircode_routing_areas_in_Ireland and
// https://autoaddress.com/eircode-routing-keys/, which agree. Eircode's own example, A65 F4E2, uses a key that is
// not on the list and is not reported.
func init() {
	Register(Detector{
		Name:        "ie-eircode",
		Label:       "Irish Eircode",
		Country:     "IE",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:[ACDEFHKNPRTVWXY]\d{2}|D6W) ?[` + eircodeChars + `]{4}`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validEircode,
		Keywords:    []string{"eircode", "éirchód"},
		Concepts:    []Concept{Address},
		Base:        domain.Low,
		Validation:  "routing key on Eircode's list of 139",
		Formatted:   regexp.MustCompile(`^\S{3} \S{4}$`),
		Needs:       Needs{Check: maybeEircode},
	})
}

// eircodeRoutingKeys are the 139 routing keys, one per principal post town or Dublin district.
var eircodeRoutingKeys = words(`A41 A42 A45 A63 A67 A75 A81 A82 A83 A84 A85 A86 A91 A92 A94 A96 A98 C15
	D01 D02 D03 D04 D05 D06 D07 D08 D09 D10 D11 D12 D13 D14 D15 D16 D17 D18 D20 D22 D24 D6W
	E21 E25 E32 E34 E41 E45 E53 E91 F12 F23 F26 F28 F31 F35 F42 F45 F52 F56 F91 F92 F93 F94
	H12 H14 H16 H18 H23 H53 H54 H62 H65 H71 H91 K32 K34 K36 K45 K56 K67 K78 N37 N39 N41 N91
	P12 P14 P17 P24 P25 P31 P32 P36 P43 P47 P51 P56 P61 P67 P72 P75 P81 P85
	R14 R21 R32 R35 R42 R45 R51 R56 R93 R95 T12 T23 T34 T45 T56 V14 V15 V23 V31 V35 V42 V92 V93 V94 V95
	W12 W23 W34 W91 X35 X42 X91 Y14 Y21 Y25 Y34 Y35`)

var eircodeShape = regexp.MustCompile(`^[A-Z0-9]{3}[` + eircodeChars + `]{4}$`)

func validEircode(s string) bool {
	if len(s) != 7 || !eircodeShape.MatchString(s) || !eircodeRoutingKeys[s[:3]] {
		return false
	}
	// A unique identifier of one repeated character (D02 0000) is a placeholder.
	return strings.Trim(s[3:], s[3:4]) != ""
}

// maybeEircode looks for a routing key on the list.
func maybeEircode(line string) bool {
	for i := 0; i+2 < len(line); i++ {
		if line[i] >= 'A' && line[i] <= 'Y' && isASCIIDigit(line[i+1]) && (isASCIIDigit(line[i+2]) || line[i+2] == 'W') && eircodeRoutingKeys[line[i:i+3]] {
			return true
		}
	}
	return false
}
