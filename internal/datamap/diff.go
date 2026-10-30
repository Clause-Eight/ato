package datamap

import "sort"

// Change is something a newer map has that an older one did not.
type Change struct {
	Component string
	Kind      string
	Name      string
}

// Kinds of change.
const (
	NewCategory  = "category"
	NewSpecial   = "special-category"
	NewRecipient = "recipient"
	NewSink      = "sink"
	NewStore     = "store"
	// NewInfra is a Terraform data store linked to personal data.
	NewInfra = "infrastructure"
)

// Diff lists what is new in current compared with previous: categories, special categories, stores and
// recipients per component, and sinks by category. Removals are not changes a reviewer must approve.
func Diff(previous, current Map) []Change {
	old := facts(previous)
	var out []Change
	for _, f := range keys(facts(current)) {
		if !old[f] {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Component != b.Component {
			return a.Component < b.Component
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return out
}

func facts(m Map) map[Change]bool {
	out := map[Change]bool{}
	for _, c := range m.Components {
		for _, s := range c.Stores {
			out[Change{c.Name, NewStore, s.Name}] = true
			for _, cat := range s.Categories {
				out[Change{c.Name, NewCategory, cat}] = true
			}
			for _, sp := range s.Special {
				out[Change{c.Name, NewSpecial, sp}] = true
			}
		}
		for _, r := range c.Recipients {
			out[Change{c.Name, NewRecipient, r.Name}] = true
		}
		for _, s := range c.Infra {
			if s.PersonalData {
				out[Change{c.Name, NewInfra, s.Address}] = true
			}
		}
		for _, s := range c.Sinks {
			out[Change{c.Name, NewSink, s.Category}] = true
		}
	}
	return out
}

func keys(m map[Change]bool) []Change {
	out := make([]Change, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Comparison is everything that differs between two maps, for a person reviewing them: what was added, what was
// removed, and stores that lost a protection they had.
type Comparison struct {
	Added    []Change
	Removed  []Change
	Weakened []Weakening
}

// Weakening is a store that had a protection in the older map and lacks it in the newer one.
type Weakening struct {
	Component string
	Store     string
	Lost      string
}

// Protections a store can lose.
const (
	LostEncryption = "encryption"
	LostRetention  = "retention"
	LostEEA        = "eea-region"
	LostErasure    = "erasure-path"
	LostExpiry     = "retention-path"
)

// Compare lists what was added and removed between previous and current, and the stores that lost encryption,
// a retention rule or an EU region in Terraform, or an erasure or retention path in the code.
func Compare(previous, current Map) Comparison {
	out := Comparison{Added: Diff(previous, current), Removed: Diff(current, previous)}
	type key struct{ component, store string }
	oldInfra, oldStores := map[key]Infra{}, map[key]Store{}
	for _, c := range previous.Components {
		for _, s := range c.Infra {
			oldInfra[key{c.Name, s.Address}] = s
		}
		for _, s := range c.Stores {
			oldStores[key{c.Name, s.Name}] = s
		}
	}
	lost := func(before, after string) bool { return before == "yes" && (after == "no" || after == "unknown") }
	for _, c := range current.Components {
		for _, s := range c.Infra {
			was, ok := oldInfra[key{c.Name, s.Address}]
			if !ok {
				continue
			}
			for _, p := range []struct{ before, after, lost string }{
				{was.Encrypted, s.Encrypted, LostEncryption},
				{was.Retention, s.Retention, LostRetention},
				{was.InEEA, s.InEEA, LostEEA},
			} {
				if lost(p.before, p.after) {
					out.Weakened = append(out.Weakened, Weakening{c.Name, s.Address, p.lost})
				}
			}
		}
		for _, s := range c.Stores {
			was, ok := oldStores[key{c.Name, s.Name}]
			if !ok || !was.Assessed || !s.Assessed {
				continue
			}
			if len(was.Erasure) > 0 && len(s.Erasure) == 0 {
				out.Weakened = append(out.Weakened, Weakening{c.Name, s.Name, LostErasure})
			}
			if len(was.Retention) > 0 && len(s.Retention) == 0 {
				out.Weakened = append(out.Weakened, Weakening{c.Name, s.Name, LostExpiry})
			}
		}
	}
	return out
}

// Empty says whether nothing differs.
func (c Comparison) Empty() bool {
	return len(c.Added) == 0 && len(c.Removed) == 0 && len(c.Weakened) == 0
}
