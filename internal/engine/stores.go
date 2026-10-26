package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/lifecycle"
)

// Detectors for stores of personal data the code never erases or expires. Both are off by default: code can
// erase data in ways a static reader does not see (a stored procedure, an admin tool, a vendor's console).
const (
	NoErasureDetector   = "no-erasure-path"
	NoRetentionDetector = "no-retention"
)

// storeNames lists the tables and models that hold personal data, outside test paths, in a stable order.
func storeNames(entries []domain.InventoryEntry) ([]string, map[string]domain.InventoryEntry) {
	first := map[string]domain.InventoryEntry{}
	var names []string
	for _, e := range entries {
		name := e.Field.Container
		if name == "" || IsTestPath(e.Field.Location.Path) {
			continue
		}
		if _, ok := first[name]; !ok {
			first[name] = e
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, first
}

func assessLifecycle(entries []domain.InventoryEntry, evidence []lifecycle.Evidence) []domain.StoreLifecycle {
	names, _ := storeNames(entries)
	status := lifecycle.Assess(names, evidence)
	out := make([]domain.StoreLifecycle, 0, len(names))
	for _, n := range names {
		st := status[n]
		out = append(out, domain.StoreLifecycle{Store: n, Erasure: removals(st.Erasure), Retention: removals(st.Retention)})
	}
	return out
}

func removals(es []lifecycle.Evidence) []domain.Removal {
	out := make([]domain.Removal, 0, len(es))
	for _, e := range es {
		out = append(out, domain.Removal{How: e.How, Location: e.Location})
	}
	return out
}

func lifecycleFindings(entries []domain.InventoryEntry, stores []domain.StoreLifecycle, req Request) []domain.Finding {
	_, first := storeNames(entries)
	categories := map[string][]string{}
	for _, e := range entries {
		c := string(e.Classification.Category)
		if e.Classification.Special != "" {
			c = string(e.Classification.Special)
		}
		if !containsString(categories[e.Field.Container], c) {
			categories[e.Field.Container] = append(categories[e.Field.Container], c)
		}
	}
	var out []domain.Finding
	add := func(detector, label, reason string, st domain.StoreLifecycle) {
		e := first[st.Store]
		cats := categories[st.Store]
		sort.Strings(cats)
		out = append(out, domain.Finding{
			Kind:        domain.KindStore,
			Category:    e.Classification.Category,
			Detector:    detector,
			Label:       label,
			Country:     domain.CrossBorder,
			Location:    e.Field.Location,
			Masked:      st.Store,
			Confidence:  domain.Medium,
			Reasons:     []string{reason, fmt.Sprintf("holds %s", strings.Join(cats, ", "))},
			Fingerprint: fingerprint(detector, e.Field.Location.Path, st.Store, 0),
		})
	}
	for _, st := range stores {
		if req.ErasureFindings && len(st.Erasure) == 0 {
			add(NoErasureDetector, "Personal data with no erasure path (GDPR Art. 17)", fmt.Sprintf("no delete or anonymising update of %s found in the code", st.Store), st)
		}
		if req.RetentionFindings && len(st.Retention) == 0 {
			add(NoRetentionDetector, "Personal data kept with no retention limit (GDPR Art. 5(1)(e))", fmt.Sprintf("no time-based delete, TTL or partition drop for %s found in the code", st.Store), st)
		}
	}
	return out
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
