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
