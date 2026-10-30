// Package datamap builds a data map from what a scan found: per component, the stores of personal data and
// their categories, where personal data is logged, and the third parties the code sends data to. It is the
// draft of a record of processing activities (GDPR Art. 30). It imports only domain.
package datamap

import (
	"path"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Map is the data map of one code base. Every claim keeps the file and line it came from.
type Map struct {
	Components []Component
}

// Component is a part of the code base: a service, an app or a top-level folder.
type Component struct {
	Name       string
	Stores     []Store
	Sinks      []Sink
	Recipients []Recipient
	Infra      []Infra
}

// Infra is a data store declared in Terraform. Settings are "yes", "no", "unknown" or "" when they do not apply.
type Infra struct {
	Address      string
	Service      string
	Regions      []string
	InEEA        string
	Encrypted    string
	Retention    string
	Public       string
	PersonalData bool
	// Module is the source of a module call that is not read inside; empty for resources.
	Module   string
	Location domain.Location
}

// Store is a table, model or message that holds personal data. Special lists the Art. 9 categories, which are
// not repeated in Categories.
type Store struct {
	Name       string
	Categories []string
	Special    []string
	Fields     []Field
	// Assessed says whether erasure and retention were looked for; Erasure and Retention are where the code
	// removes the store's data.
	Assessed  bool
	Erasure   []domain.Removal
	Retention []domain.Removal
}

// Field is one personal-data field of a store.
type Field struct {
	Name     string
	Category string
	Special  string
	Location domain.Location
}

// Sink is a place where personal data is written to a log, an error tracker or analytics.
type Sink struct {
	Label    string
	Category string
	Location domain.Location
}

// Recipient is a third party the component names, with every place it is named.
type Recipient struct {
	Name     string
	Purpose  string
	EEA      bool
	Evidence []domain.Location
}

// Input is what a scan collected.
type Input struct {
	Inventory  []domain.InventoryEntry
	Findings   []domain.Finding
	Recipients []domain.RecipientUse
	Lifecycle  []domain.StoreLifecycle
	Infra      []domain.Store
}

// Build groups the input by component.
func Build(in Input) Map {
	comps := map[string]*Component{}
	get := func(p string) *Component {
		name := ComponentOf(p)
		c := comps[name]
		if c == nil {
			c = &Component{Name: name}
			comps[name] = c
		}
		return c
	}
	stores := map[string]map[string]*Store{}
	for _, e := range in.Inventory {
		c := get(e.Field.Location.Path)
		name := e.Field.Container
		if name == "" {
			name = path.Base(e.Field.Location.Path)
		}
		if stores[c.Name] == nil {
			stores[c.Name] = map[string]*Store{}
		}
		s := stores[c.Name][name]
		if s == nil {
			s = &Store{Name: name}
			stores[c.Name][name] = s
		}
		s.Fields = append(s.Fields, Field{Name: e.Field.Name, Category: string(e.Classification.Category), Special: string(e.Classification.Special), Location: e.Field.Location})
		if e.Classification.Special != "" {
			s.Special = addUnique(s.Special, string(e.Classification.Special))
		} else {
			s.Categories = addUnique(s.Categories, string(e.Classification.Category))
		}
	}
	for _, f := range in.Findings {
		if f.Kind != domain.KindSink {
			continue
		}
		c := get(f.Location.Path)
		c.Sinks = append(c.Sinks, Sink{Label: f.Label, Category: string(f.Category), Location: f.Location})
	}
	recipients := map[string]map[string]*Recipient{}
	for _, u := range in.Recipients {
		c := get(u.Location.Path)
		if recipients[c.Name] == nil {
			recipients[c.Name] = map[string]*Recipient{}
		}
		r := recipients[c.Name][u.Name]
		if r == nil {
			r = &Recipient{Name: u.Name, Purpose: u.Purpose, EEA: u.EEA}
			recipients[c.Name][u.Name] = r
		}
		r.Evidence = append(r.Evidence, u.Location)
	}
	for _, s := range in.Infra {
		c := get(s.Location.Path)
		c.Infra = append(c.Infra, Infra{
			Address: s.Type + "." + s.Name, Service: s.Service, Regions: s.Regions, InEEA: string(s.InEEA.State),
			Encrypted: string(s.Encryption.State), Retention: string(s.Retention.State), Public: string(s.Public.State),
			PersonalData: s.PersonalData, Module: s.Module, Location: s.Location,
		})
	}
	life := map[string]domain.StoreLifecycle{}
	for _, l := range in.Lifecycle {
		life[l.Store] = l
	}
	var out Map
	for name, c := range comps {
		for _, s := range stores[name] {
			if l, ok := life[s.Name]; ok {
				s.Assessed, s.Erasure, s.Retention = true, l.Erasure, l.Retention
			}
			sort.Strings(s.Categories)
			sort.Strings(s.Special)
			c.Stores = append(c.Stores, *s)
		}
		sort.Slice(c.Stores, func(i, j int) bool { return c.Stores[i].Name < c.Stores[j].Name })
		for _, r := range recipients[name] {
			c.Recipients = append(c.Recipients, *r)
		}
		sort.Slice(c.Recipients, func(i, j int) bool { return c.Recipients[i].Name < c.Recipients[j].Name })
		out.Components = append(out.Components, *c)
	}
	sort.Slice(out.Components, func(i, j int) bool { return out.Components[i].Name < out.Components[j].Name })
	return out
}

// groupFolders hold one component per subfolder.
var groupFolders = map[string]bool{"services": true, "apps": true, "packages": true, "cmd": true, "modules": true, "microservices": true, "components": true, "functions": true, "lambdas": true, "libs": true}

// ComponentOf names the component a path belongs to: the subfolder of a grouping folder such as services/ or
// apps/, otherwise the top-level folder, and "." for files at the root.
func ComponentOf(p string) string {
	if i := strings.Index(p, "!/"); i >= 0 {
		p = p[:i]
	}
	parts := strings.Split(path.Clean(strings.TrimPrefix(p, "./")), "/")
	switch {
	case len(parts) == 1:
		return "."
	case len(parts) > 2 && groupFolders[parts[0]]:
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// Categories lists every category in the map, special categories as "special-category/<name>".
func (m Map) Categories() []string {
	var out []string
	for _, c := range m.Components {
		for _, s := range c.Stores {
			for _, cat := range s.Categories {
				out = addUnique(out, cat)
			}
			for _, sp := range s.Special {
				out = addUnique(out, string(domain.CategorySpecial)+"/"+sp)
			}
		}
	}
	sort.Strings(out)
	return out
}

func addUnique(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}
