package classify

import (
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

func TestField(t *testing.T) {
	for name, want := range map[string]domain.Classification{
		"diagnosis":               {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"criminal_record":         {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"hasConviction":           {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"criminal_offence":        {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"offense_history":         {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"police_record":           {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"prisonSentence":          {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"background_check_result": {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"cazier_judiciar":         {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"führungszeugnis":         {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"casier_judiciaire":       {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"antecedentes_penales":    {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"strafblad":               {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"niekaralnosc":            {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"erkolcsi_bizonyitvany":   {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"rikosrekisteriote":       {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"belastningsregister":     {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"съдимост":                {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"ποινικό_μητρώο":          {Category: domain.CategorySpecial, Special: domain.SpecialCriminal},
		"conviction_religieuse":   {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"religious_affiliation":   {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"kirchensteuer":           {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"political_views":         {Category: domain.CategorySpecial, Special: domain.SpecialPolitical},
		"politicka_orientace":     {Category: domain.CategorySpecial, Special: domain.SpecialPolitical},
		"ethnicBackground":        {Category: domain.CategorySpecial, Special: domain.SpecialRacialEthnic},
		"etnicitate":              {Category: domain.CategorySpecial, Special: domain.SpecialRacialEthnic},
		"union_dues":              {Category: domain.CategorySpecial, Special: domain.SpecialTradeUnion},
		"skladka_zwiazkowa":       {Category: domain.CategorySpecial, Special: domain.SpecialTradeUnion},
		"sexual_behaviour":        {Category: domain.CategorySpecial, Special: domain.SpecialSexLife},
		"preferinta_sexuala":      {Category: domain.CategorySpecial, Special: domain.SpecialSexLife},
		"patientDiagnosis":        {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"blood_type":              {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"BloodGroup":              {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"religion":                {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"religie":                 {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"Konfession":              {Category: domain.CategorySpecial, Special: domain.SpecialReligious},
		"trade_union_member":      {Category: domain.CategorySpecial, Special: domain.SpecialTradeUnion},
		"sindicat":                {Category: domain.CategorySpecial, Special: domain.SpecialTradeUnion},
		"ethnic_origin":           {Category: domain.CategorySpecial, Special: domain.SpecialRacialEthnic},
		"sexual_orientation":      {Category: domain.CategorySpecial, Special: domain.SpecialSexLife},
		"orientare_sexuala":       {Category: domain.CategorySpecial, Special: domain.SpecialSexLife},
		"politicalParty":          {Category: domain.CategorySpecial, Special: domain.SpecialPolitical},
		"dna_sample":              {Category: domain.CategorySpecial, Special: domain.SpecialGenetic},
		"face_template":           {Category: domain.CategorySpecial, Special: domain.SpecialBiometric},
		"fingerabdruck":           {Category: domain.CategorySpecial, Special: domain.SpecialBiometric},
		"διάγνωση":                {Category: domain.CategorySpecial, Special: domain.SpecialHealth},
		"first_name":              {Category: domain.CategoryName},
		"firstname":               {Category: domain.CategoryName},
		"prenume":                 {Category: domain.CategoryName},
		"email_address":           {Category: domain.CategoryContact},
		"phoneNumber":             {Category: domain.CategoryContact},
		"cnp":                     {Category: domain.CategoryIdentifier},
		"pesel":                   {Category: domain.CategoryIdentifier},
		"iban":                    {Category: domain.CategoryFinancial},
		"date_of_birth":           {Category: domain.CategoryBirth},
		"dateOfBirth":             {Category: domain.CategoryBirth},
		"geburtsdatum":            {Category: domain.CategoryBirth},
		"ip_address":              {Category: domain.CategoryOnline},
		"gender":                  {Category: domain.CategoryDemographic},
		"street_address":          {Category: domain.CategoryAddress},
		"latitude":                {Category: domain.CategoryLocation},
	} {
		got, ok := Field(name)
		if !ok || got.Category != want.Category || got.Special != want.Special {
			t.Errorf("%s: got %+v %v, want %s/%s", name, got, ok, want.Category, want.Special)
		}
	}
}

func TestFieldsThatAreNotPersonalData(t *testing.T) {
	for _, name := range []string{"id", "created_at", "health_check", "healthCheck", "trace_id", "status", "order_total", "race_condition", "union_type", "card_title", "unionAll", "price", "description", "condition", "Health", "dns", "dns_search", "diagnosticInfo", "medical", "symptoms", "gen", "hit",
		"sentence", "sentence_id", "first_sentence", "sentenceCount", "offense_rating", "world_view", "background_color", "check_result", "record_id", "police_station", "domain", "dom_node"} {
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
