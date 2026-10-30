package datamap

import (
	"testing"

	"github.com/Clause-Eight/ato/internal/domain"
)

func entry(p, container, name string, cat domain.DataCategory, special domain.SpecialCategory) domain.InventoryEntry {
	return domain.InventoryEntry{
		Field:          domain.Field{Name: name, Container: container, Location: domain.Location{Path: p, Line: 1}},
		Classification: domain.Classification{Category: cat, Special: special},
	}
}

func TestComponentOf(t *testing.T) {
	for p, want := range map[string]string{
		"main.go":                           ".",
		"api/handlers/user.go":              "api",
		"services/billing/db/001.sql":       "services/billing",
		"services/README.md":                "services",
		"apps/web/src/a.ts":                 "apps/web",
		"dump.tar.gz!/services/x/users.csv": ".",
		"./db/migrations/001_patients.sql":  "db",
	} {
		if got := ComponentOf(p); got != want {
			t.Errorf("%s: got %s, want %s", p, got, want)
		}
	}
}

func TestBuildGroupsByComponentAndStore(t *testing.T) {
	m := Build(Input{
		Inventory: []domain.InventoryEntry{
			entry("services/clinic/db/001.sql", "patients", "first_name", domain.CategoryName, ""),
			entry("services/clinic/db/001.sql", "patients", "diagnosis", domain.CategorySpecial, domain.SpecialHealth),
			entry("services/billing/models.py", "Invoice", "iban", domain.CategoryFinancial, ""),
		},
		Findings: []domain.Finding{
			{Kind: domain.KindSink, Label: "Personal data written to a log", Category: domain.CategoryContact, Location: domain.Location{Path: "services/billing/api.py", Line: 9}},
			{Kind: domain.KindValue, Location: domain.Location{Path: "services/billing/x.py"}},
		},
		Recipients: []domain.RecipientUse{
			{Name: "Stripe", Purpose: "payments", Location: domain.Location{Path: "services/billing/requirements.txt", Line: 2}},
			{Name: "Stripe", Purpose: "payments", Location: domain.Location{Path: "services/billing/pay.py", Line: 1}},
		},
	})
	if len(m.Components) != 2 || m.Components[0].Name != "services/billing" || m.Components[1].Name != "services/clinic" {
		t.Fatalf("components: %+v", m.Components)
	}
	billing, clinic := m.Components[0], m.Components[1]
	if len(billing.Sinks) != 1 || len(billing.Recipients) != 1 || len(billing.Recipients[0].Evidence) != 2 {
		t.Errorf("billing: %+v", billing)
	}
	if len(clinic.Stores) != 1 || clinic.Stores[0].Special[0] != "health" || len(clinic.Stores[0].Categories) != 1 {
		t.Errorf("clinic: %+v", clinic)
	}
	want := []string{"financial", "name", "special-category/health"}
	if got := m.Categories(); len(got) != len(want) {
		t.Errorf("categories: %v", got)
	}
}

func TestDiffListsOnlyWhatIsNew(t *testing.T) {
	before := Build(Input{Inventory: []domain.InventoryEntry{entry("api/models.py", "User", "email", domain.CategoryContact, "")}})
	after := Build(Input{
		Inventory: []domain.InventoryEntry{
			entry("api/models.py", "User", "email", domain.CategoryContact, ""),
			entry("api/models.py", "User", "religion", domain.CategorySpecial, domain.SpecialReligious),
		},
		Recipients: []domain.RecipientUse{{Name: "OpenAI", Location: domain.Location{Path: "api/ai.py"}}},
	})
	got := Diff(before, after)
	want := []Change{{"api", NewRecipient, "OpenAI"}, {"api", NewSpecial, "religious-or-philosophical-beliefs"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if d := Diff(after, before); len(d) != 0 {
		t.Errorf("removals are not changes: %+v", d)
	}
}

func TestCompareListsRemovalsAndLostProtections(t *testing.T) {
	bucket := func(eea, enc domain.Tri) domain.Store {
		return domain.Store{Type: "aws_s3_bucket", Name: "exports", Regions: []string{"eu-west-1"}, InEEA: domain.Setting{State: eea}, Encryption: domain.Setting{State: enc}, Retention: domain.Setting{State: domain.Yes}, PersonalData: true, Location: domain.Location{Path: "infra/main.tf", Line: 1}}
	}
	erase := []domain.Removal{{How: "SQL DELETE", Location: domain.Location{Path: "api/repo.sql", Line: 3}}}
	before := Build(Input{
		Inventory: []domain.InventoryEntry{entry("api/models.py", "User", "email", domain.CategoryContact, ""), entry("api/models.py", "Lead", "phone", domain.CategoryContact, "")},
		Infra:     []domain.Store{bucket(domain.Yes, domain.Yes)},
		Lifecycle: []domain.StoreLifecycle{{Store: "User", Erasure: erase, Retention: erase}},
	})
	after := Build(Input{
		Inventory: []domain.InventoryEntry{entry("api/models.py", "User", "email", domain.CategoryContact, "")},
		Infra:     []domain.Store{bucket(domain.No, domain.Unknown)},
		Lifecycle: []domain.StoreLifecycle{{Store: "User", Erasure: nil, Retention: erase}},
	})
	got := Compare(before, after)
	if len(got.Added) != 0 || len(got.Removed) != 1 || got.Removed[0] != (Change{"api", NewStore, "Lead"}) {
		t.Errorf("added %+v, removed %+v", got.Added, got.Removed)
	}
	want := []Weakening{{"infra", "aws_s3_bucket.exports", LostEncryption}, {"infra", "aws_s3_bucket.exports", LostEEA}, {"api", "User", LostErasure}}
	if len(got.Weakened) != len(want) {
		t.Fatalf("weakened: %+v", got.Weakened)
	}
	for _, w := range want {
		found := false
		for _, g := range got.Weakened {
			found = found || g == w
		}
		if !found {
			t.Errorf("missing %+v in %+v", w, got.Weakened)
		}
	}
	if !Compare(after, after).Empty() {
		t.Error("a map compared with itself must be empty")
	}
}
