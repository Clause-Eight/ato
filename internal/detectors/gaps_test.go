package detectors

import "testing"

// The detectors added to close the coverage gaps. Valid values are the published examples of each source (or of
// python-stdnum's documentation, for the rules it implements); the generators and a 520,000-value comparison with
// python-stdnum 1.20 check the rest.
func TestCoverageGapDetectors(t *testing.T) {
	cases := []struct {
		detector, why, value string
		want                 bool
	}{
		{"hu-personal-id", "born 1985, weights 1 to 10", "18501011231", true},
		{"hu-personal-id", "born 31 December 1996, still the old weights", "19612310019", true},
		{"hu-personal-id", "born 2005, weights 10 down to 1", "30501011232", true},
		{"hu-personal-id", "born 1997, the reversed weights", "29703154568", true},
		{"hu-personal-id", "born 1997 with the old weights", "29703154563", false},
		{"hu-personal-id", "printed with spaces", "1 850101 1231", true},
		{"hu-personal-id", "first digit 9", "98501011231", false},
		{"hu-personal-id", "30 February", "18502301234", false},

		{"pt-niss", "the validator's own test value", "12345678902", true},
		{"pt-niss", "wrong check digit", "12345678903", false},
		{"pt-niss", "first digit 3", "32345678902", false},

		{"de-kvnr", "worked example", "A123456780", true},
		{"de-kvnr", "wrong check digit", "A123456781", false},
		{"de-kvnr", "another letter", "B123456780", false},

		{"gb-utr", "HMRC example", "1955839661", true},
		{"gb-utr", "printed in two groups", "19558 39661", true},
		{"gb-utr", "wrong check digit", "2955839661", false},

		{"gb-upn", "Department for Education example", "B801200005001", true},
		{"gb-upn", "local authority 999 does not exist", "B999200005001", false},
		{"gb-upn", "wrong check letter", "C801200005001", false},

		{"nl-onderwijsnummer", "DUO example", "101222331", true},
		{"nl-onderwijsnummer", "passes the BSN elfproef instead", "101222332", false},
		{"nl-onderwijsnummer", "not starting with 10", "111222331", false},

		{"cz-bank-account", "prefix, root and bank", "19-2000145399/0800", true},
		{"cz-bank-account", "without a prefix", "2000145399/0800", true},
		{"cz-bank-account", "bank code not assigned", "2000145399/0801", false},
		{"cz-bank-account", "root fails mod 11", "2000145398/0800", false},
		{"cz-bank-account", "root of zeros", "0000000000/0800", false},

		{"es-ccc", "Wikipedia example", "12341234161234567890", true},
		{"es-ccc", "printed in groups", "1234 1234 16 1234567890", true},
		{"es-ccc", "wrong second check digit", "12341234171234567890", false},

		{"no-kontonr", "ECBS example", "86011117947", true},
		{"no-kontonr", "printed 4.2.5", "8601.11.17947", true},
		{"no-kontonr", "wrong check digit", "86011117948", false},

		{"pl-nrb", "the account of the Polish IBAN example", "61109010140000071219812874", true},
		{"pl-nrb", "printed in groups", "61 1090 1014 0000 0712 1981 2874", true},
		{"pl-nrb", "wrong check digits", "62109010140000071219812874", false},

		{"rf-creditor-reference", "ISO 11649 example", "RF18539007547034", true},
		{"rf-creditor-reference", "printed in groups", "RF18 5390 0754 7034", true},
		{"rf-creditor-reference", "with a letter", "RF185390075470Y", true},
		{"rf-creditor-reference", "wrong check digits", "RF19539007547034", false},

		{"ch-qr-reference", "QR-bill example", "210000000003139471430009017", true},
		{"ch-qr-reference", "printed in groups", "21 00000 00003 13947 14300 09017", true},
		{"ch-qr-reference", "wrong check digit", "210000000003139471430009018", false},
		{"ch-qr-reference", "all zeros", "000000000000000000000000000", false},

		{"iccid", "19 digits, United Kingdom", "8944500102198304826", true},
		{"iccid", "Luhn fails", "8944500102198304827", false},

		{"es-cups", "Wikipedia example", "ES1234123456789012JY", true},
		{"es-cups", "with a border point", "ES1234123456789012JY1F", true},
		{"es-cups", "wrong check letters", "ES1234123456789012JZ", false},

		{"es-cadastral-reference", "Catastro example, urban", "7837301VG8173B0001TT", true},
		{"es-cadastral-reference", "printed in groups", "7837301 VG8173B 0001 TT", true},
		{"es-cadastral-reference", "a check letter after Ñ in the alphabet", "2926126PG0642Q9287DZ", true},
		{"es-cadastral-reference", "wrong check letter", "7837301VG8173B0001TU", false},
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
