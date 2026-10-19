package detectors

import (
	"testing"
)

func TestPostcode(t *testing.T) {
	d := detector(t, "postcode")
	if !d.NeedsKeyword {
		t.Error("postcode must need a keyword")
	}
	checkCases(t, "postcode", map[string]bool{
		"10115":    true,  // DE
		"1010":     true,  // AT, HU
		"0900":     true,  // DK
		"0700":     false, // below every range
		"1234 AB":  true,  // NL
		"1234AB":   true,
		"1234 SA":  false, // unused letter pair
		"0123 AB":  false,
		"00-950":   true, // PL
		"00-000":   false,
		"1000-001": true, // PT
		"0999-001": false,
		"110 00":   true, // CZ
		"000 00":   false,
		"A65 F4E2": true, // IE
		"D6W 1234": true,
		"A65F4E2":  true,
		"B65 F4E2": false, // B is not used
		"A65 F4B2": false,
		"VLT 1117": true, // MT
		"XYZ 1117": false,
		"L-1234":   true, // LU
		"LV-1050":  true,
		"LT-01100": true,
		"LT-0110":  false,
		"D-10115":  true,
		"D-1011":   false,
		"DE-00999": false,
		"ES-53001": false, // province 53 does not exist
		"ES-28001": true,
		"HR-60000": false,
		"SK-81101": true,
		"CZ-81101": false,
		"XX-12345": false,
		"010011":   true, // RO
		"001234":   false,
		"00000":    false,
	})
	if !digitsOrEircode("addr: A65 F4E2") || !digitsOrEircode("plz 10115") || digitsOrEircode("v1.2.3 ok") {
		t.Error("the postcode prefilter is wrong")
	}
}

func TestStreetAddress(t *testing.T) {
	d := detector(t, "street-address")
	cases := []string{
		"Strada Lalelelor 12", "Bulevardul Unirii nr. 5", "Calea Victoriei 25", "ul. Kwiatowa 3", "ul. Kwiatowa 3/5",
		"Hauptstraße 5", "Karl-Marx-Straße 12a", "Berliner Straße 7", "Hauptstr. 9", "rue de la Paix 10",
		"10 rue de la Paix", "Calle Mayor 7", "Calle Mayor, 7", "Via Roma 1", "Rua Augusta, 100", "Kerkstraat 12",
		"Fő utca 1", "Brīvības iela 12", "Gedimino pr. 9", "Prešernova ulica 3", "Drottninggatan 5",
		"Mannerheimintie 5", "12 Main Street", "Triq il-Kbira 5", "ул. Витоша 15", "οδός Ερμού 10",
	}
	for i, c := range cases {
		line := "address: " + c + "."
		got := d.Pattern.FindString(line)
		if got != c || !d.Validate(got) {
			t.Errorf("street case %d: found %d of %d characters", i, len(got), len(c))
		}
		if !d.Needs.Admits(line, 9) {
			t.Errorf("street case %d: the prefilter drops the line", i)
		}
	}
	for i, c := range []string{
		"sent via Kafka 3 times", "Strada 12", "return true 5", "Hauptstraße", "ul. kwiatowa 3",
		"Strada Lalelelor 0", "x := strings.Index(s, 5)",
	} {
		if got := d.Pattern.FindString(c); got != "" && d.Validate(got) {
			t.Errorf("non-address case %d matched", i)
		}
	}
	if d.NeedsKeyword {
		t.Error("street-address keeps the keyword optional: the street word is the context")
	}
}
