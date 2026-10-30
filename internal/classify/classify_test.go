package classify

import (
	"testing"

	"github.com/Clause-Eight/ato/internal/domain"
)

func TestField(t *testing.T) {
	for name, want := range map[string]domain.Classification{
		"diagnosis":          {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"patientDiagnosis":   {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"blood_type":         {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"BloodGroup":         {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"religion":           {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"religie":            {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"Konfession":         {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"trade_union_member": {Category: domain.CategorySpecial, Special: domain.SpecialTradeUnion},
		"sindicat":           {Category: domain.CategorySpecial, Special: domain.SpecialTradeUnion},
		"ethnic_origin":      {Category: domain.CategorySpecial, Special: domain.SpecialRacialEthnic},
		"sexual_orientation": {Category: domain.CategorySpecial, Special: domain.SpecialSexLife},
		"orientare_sexuala":  {Category: domain.CategorySpecial, Special: domain.SpecialSexLife},
		"politicalParty":     {Category: domain.CategorySpecial, Special: domain.SpecialPolitical},
		"dna_sample":         {Category: domain.CategorySpecial, Special: domain.SpecialGenetic},
		"face_template":      {Category: domain.CategorySpecial, Special: domain.SpecialBiometric},
		"fingerabdruck":      {Category: domain.CategorySpecial, Special: domain.SpecialBiometric},
		"διάγνωση":           {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"first_name":         {Category: domain.CategoryName},
		"firstname":          {Category: domain.CategoryName},
		"prenume":            {Category: domain.CategoryName},
		"email_address":      {Category: domain.CategoryContact},
		"phoneNumber":        {Category: domain.CategoryContact},
		"cnp":                {Category: domain.CategoryIdentifier},
		"pesel":              {Category: domain.CategoryIdentifier},
		"iban":               {Category: domain.CategoryFinancial},
		"date_of_birth":      {Category: domain.CategoryBirth},
		"dateOfBirth":        {Category: domain.CategoryBirth},
		"geburtsdatum":       {Category: domain.CategoryBirth},
		"ip_address":         {Category: domain.CategoryOnline},
		"gender":             {Category: domain.CategoryDemographic},
		"street_address":     {Category: domain.CategoryAddress},
		"latitude":           {Category: domain.CategoryLocation},
	} {
		got, ok := Field(name)
		if !ok || got.Category != want.Category || got.Special != want.Special {
			t.Errorf("%s: got %+v %v, want %s/%s", name, got, ok, want.Category, want.Special)
		}
	}
}

func TestFieldsThatAreNotPersonalData(t *testing.T) {
	for _, name := range []string{"id", "created_at", "health_check", "healthCheck", "trace_id", "status", "order_total", "race_condition", "union_type", "card_title", "unionAll", "price", "description", "condition", "Health", "dns", "dns_search", "diagnosticInfo", "medical", "symptoms", "gen", "hit"} {
		if got, ok := Field(name); ok {
			t.Errorf("%s classified as %+v", name, got)
		}
	}
}

func TestTokenize(t *testing.T) {
	got := tokenize("patientHIVStatus_v2")
	want := []string{"patient", "hiv", "status", "v2"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}
