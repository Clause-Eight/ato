package detectors

import (
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Identifiers with a layout and no control digit. Valid values are the published examples of each source where
// there is one, and made-up values of the right shape otherwise; they belong to nobody.
func TestFormatOnlyDetectors(t *testing.T) {
	cases := []struct {
		detector, why, value string
		want                 bool
	}{
		{"fi-veronumero", "python-stdnum example", "123456789123", true},
		{"fi-veronumero", "eleven digits", "12345678912", false},
		{"fi-veronumero", "a letter at the end", "12345678912A", false},

		{"de-stnr", "Bayern, FFF/BBB/UUUUP", "181/815/08155", true},
		{"de-stnr", "Berlin, FF/BBB/UUUUP", "21/815/08150", true},
		{"de-stnr", "Hessen, 0FF/BBB/UUUUP", "013/815/08153", true},
		{"de-stnr", "Nordrhein-Westfalen, FFF/BBBB/UUUP", "133/8150/8159", true},
		{"de-stnr", "Baden-Württemberg, FFBBB/UUUUP", "93815/08152", true},
		{"de-stnr", "Sachsen, without separators", "20112312340", true},
		{"de-stnr", "federal form, Thüringen", "4151081508156", true},
		{"de-stnr", "federal form, Bayern", "9181081508155", true},
		{"de-stnr", "federal form, Nordrhein-Westfalen", "5133081508159", true},
		{"de-stnr", "federal form without the 0 after the tax office", "4151181508156", false},
		{"de-stnr", "federal form with state code 12", "1215081508156", false},
		{"de-stnr", "nine digits", "136695978", false},
		{"de-stnr", "twelve digits", "123456789012", false},

		{"li-peid", "seven digits", "1234567", true},
		{"li-peid", "with leading zeros", "00001234567", true},
		{"li-peid", "four digits", "1234", true},
		{"li-peid", "three digits after the zeros", "0000123", false},
		{"li-peid", "thirteen digits", "1234567890123", false},

		{"ro-cei", "two letters and seven digits", "SP1236343", true},
		{"ro-cei", "with a space after the series", "SP 1236343", true},
		{"ro-cei", "six digits, the older card", "SP123634", false},
		{"ro-cei", "eight digits", "SP12363431", false},
		{"ro-cei", "a lower-case series", "sp1236343", false},

		{"hr-mbo", "HZZO's sample number", "999999993", true},
		{"hr-mbo", "nine digits", "123456789", true},
		{"hr-mbo", "the same digit nine times", "000000000", false},
		{"hr-mbo", "ten digits", "1234567890", false},

		{"imsi", "Romania", "226011234567890", true},
		{"imsi", "Germany", "262019876543210", true},
		{"imsi", "a country code outside the covered countries", "250011234567890", false},
		{"imsi", "a country code that is not European", "310150123456789", false},
		{"imsi", "fourteen digits", "26201987654321", false},
	}
	for _, c := range cases {
		d, ok := Get(c.detector)
		if !ok {
			t.Fatalf("%s is not registered", c.detector)
		}
		if got := d.Validate(d.Normalize(c.value)); got != c.want {
			t.Errorf("%s, %s: got %v, want %v", c.detector, c.why, got, c.want)
		}
		if c.want {
			if m := d.Pattern.FindString(c.value); m != c.value {
				t.Errorf("%s, %s: the pattern finds %q", c.detector, c.why, m)
			}
		}
	}
}

// Without a control digit, each needs a keyword, says it has no check, and is on by default only for an EU
// member state.
func TestFormatOnlyDetectorsNeedAKeyword(t *testing.T) {
	member := map[string]bool{}
	for _, cc := range domain.MemberStates {
		member[cc] = true
	}
	for _, name := range []string{"fi-veronumero", "de-stnr", "li-peid", "ro-cei", "hr-mbo", "imsi"} {
		d, _ := Get(name)
		if !d.NeedsKeyword {
			t.Errorf("%s has no control digit and must need a keyword", name)
		}
		if strings.Contains(d.Validation, "check") {
			t.Errorf("%s: the validation must not claim a check", name)
		}
		if d.OnByDefault && !member[d.Country] {
			t.Errorf("%s must be off by default", name)
		}
	}
	for _, name := range []string{"li-peid", "imsi"} {
		if d, _ := Get(name); d.OnByDefault {
			t.Errorf("%s matches too much to be on by default", name)
		}
	}
}
