package detectors

import (
	"strings"
	"testing"
)

func TestFold(t *testing.T) {
	for in, want := range map[string]string{
		"Numéro_Fiscal":      "numero fiscal",
		"Straße":             "strasse",
		"ADÓAZONOSÍTÓ-JEL":   "adoazonosito jel",
		"Αριθμός Ταυτότητας": "αριθμοσ ταυτοτητασ",
		"ЕГН":                "егн",
		"Łódź":               "lodz",
		"cod numeric":        "cod numeric",
	} {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEveryLanguageHasTheCoreConcepts(t *testing.T) {
	core := []Concept{NationalID, TaxID, SocialSecurity, HealthInsurance, Phone, Email, BankAccount, VAT, Passport, IDCard, BirthDate, Address, Postcode, DrivingLicence, LicencePlate, Location, Name}
	if len(Languages) != 24 {
		t.Fatalf("want the 24 official EU languages, have %d", len(Languages))
	}
	for _, lang := range Languages {
		data, err := keywordFiles.ReadFile("keywords/" + lang + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		have := map[Concept]bool{}
		for _, line := range strings.Split(string(data), "\n") {
			c, words, ok := strings.Cut(line, ":")
			if ok && !strings.HasPrefix(line, "#") && strings.TrimSpace(words) != "" {
				have[Concept(strings.TrimSpace(c))] = true
			}
		}
		for _, c := range core {
			if !have[c] {
				t.Errorf("%s.txt has no words for %s", lang, c)
			}
		}
		for c := range have {
			if ConceptWords(c) == nil {
				t.Errorf("%s.txt uses unknown concept %q", lang, c)
			}
		}
	}
}

func TestConceptWordsSkipOtherDetectorsNames(t *testing.T) {
	oib, _ := Get("hr-oib")
	pesel, _ := Get("pl-pesel")
	if contains(oib.MatchWords, "pesel") {
		t.Error("the PESEL detector's own name must not raise an OIB")
	}
	if !contains(pesel.MatchWords, "pesel") || !contains(oib.MatchWords, "osobni identifikacijski broj") {
		t.Error("own names and concept words missing")
	}
	if !contains(pesel.MatchWords, "henkilotunnus") {
		t.Error("concept words from other languages are folded and shared")
	}
}

func TestMinDigits(t *testing.T) {
	for _, name := range []string{"ro-cnp", "iban", "card", "email"} {
		d, _ := Get(name)
		want := map[string]int{"ro-cnp": 13, "iban": 10, "card": 13, "email": 0}[name]
		if d.Needs.Digits != want {
			t.Errorf("%s: min digits %d, want %d", name, d.Needs.Digits, want)
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
