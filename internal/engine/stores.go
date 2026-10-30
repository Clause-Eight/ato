package engine

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Clause-Eight/ato/internal/domain"
	"github.com/Clause-Eight/ato/internal/infra"
)

// InfraDetectors are the names of the checks on Terraform data stores. Each can be switched off in settings
// like a detector.
func InfraDetectors() []string {
	out := make([]string, 0, len(domain.InfraChecks))
	for _, c := range domain.InfraChecks {
		out = append(out, c.Detector)
	}
	return out
}

// IsInfraDetector reports whether name is a check on Terraform data stores.
func IsInfraDetector(name string) bool {
	_, ok := domain.InfraCheckFor(name)
	return ok
}

// StoreTypes lists the Terraform resource types read as data stores, by provider.
func StoreTypes() map[string][]string { return infra.Types() }

// minContainer is the shortest table or model name used to link a store, so short names such as "app" do not
// link everything.
const minContainer = 4

// linkStores marks stores whose name matches a table or model that holds personal data. A store already linked
// by a classification tag keeps that reason.
func linkStores(stores []domain.Store, entries []domain.InventoryEntry) {
	type source struct{ name, path string }
	containers := map[string]source{}
	var order []string
	for _, e := range entries {
		key := strings.Join(words(e.Field.Container), "")
		if len(key) < minContainer {
			continue
		}
		if _, ok := containers[key]; !ok {
			containers[key] = source{e.Field.Container, e.Field.Location.Path}
			order = append(order, key)
		}
	}
	for i := range stores {
		s := &stores[i]
		if s.PersonalData {
			continue
		}
		candidates := runs(append([]string{s.Name}, s.Names...))
		for _, key := range order {
			if matchesAny(key, candidates) {
				c := containers[key]
				s.PersonalData = true
				s.PersonalWhy = fmt.Sprintf("its name matches %q, which holds personal data in %s", c.name, c.path)
				break
			}
		}
	}
}

// words splits a name into lower-case words at separators and camel-case boundaries: PatientRecord and
// patient_records both give patient, record(s).
func words(name string) []string {
	var out []string
	var cur strings.Builder
	prevLower := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range name {
		switch {
		case unicode.IsUpper(r):
			if prevLower {
				flush()
			}
			cur.WriteRune(unicode.ToLower(r))
			prevLower = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur.WriteRune(r)
			prevLower = true
		default:
			flush()
			prevLower = false
		}
	}
	flush()
	return out
}

// runs are the words of each name and every run of consecutive words joined, so "eu-patient-records" offers
// patient, records and patientrecords among others.
func runs(names []string) []string {
	var out []string
	for _, n := range names {
		ws := words(n)
		for i := range ws {
			for j := i + 1; j <= len(ws); j++ {
				out = append(out, strings.Join(ws[i:j], ""))
			}
		}
	}
	return out
}

func matchesAny(key string, candidates []string) bool {
	for _, c := range candidates {
		if samePlural(key, c) {
			return true
		}
	}
	return false
}

// samePlural compares two words allowing an English plural on either side.
func samePlural(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return b == a+"s" || b == a+"es" || strings.HasSuffix(a, "y") && b == a[:len(a)-1]+"ies"
}

// storeFindings turns store settings into findings for the enabled checks. A definite answer is high when the
// store is linked to personal data and medium otherwise; an unknown answer is reported, at low confidence,
// only for linked stores. Module calls give no findings: nothing inside them is read, so they are listed in the
// inventory only.
func storeFindings(stores []domain.Store, enabled []string, files []*infra.File) []domain.Finding {
	lines := map[string][]string{}
	for _, f := range files {
		lines[f.Path] = f.Lines
	}
	var out []domain.Finding
	for i := range stores {
		s := &stores[i]
		if s.Module != "" {
			continue
		}
		for _, name := range enabled {
			c, ok := domain.InfraCheckFor(name)
			if !ok {
				continue
			}
			set, failed := judge(s, name)
			if !failed && set.State != domain.Unknown || set.State == domain.Unknown && !s.PersonalData {
				continue
			}
			out = append(out, storeFinding(s, c, set, lines[s.Location.Path]))
		}
	}
	return out
}

// judge returns the setting a check looks at and whether it definitely fails.
func judge(s *domain.Store, detector string) (domain.Setting, bool) {
	switch detector {
	case "tf-unencrypted":
		return s.Encryption, s.Encryption.State == domain.No
	case "tf-no-retention":
		return s.Retention, s.Retention.State == domain.No
	case "tf-non-eea-region":
		return s.InEEA, s.InEEA.State == domain.No
	case "tf-public":
		return s.Public, s.Public.State == domain.Yes
	}
	return domain.Setting{}, false
}

func storeFinding(s *domain.Store, c domain.InfraCheck, set domain.Setting, text []string) domain.Finding {
	loc := s.Location
	if set.Line > 0 {
		loc.Line = set.Line
	}
	f := domain.Finding{
		Kind:        domain.KindInfra,
		Detector:    c.Detector,
		Label:       c.Label,
		Country:     domain.CrossBorder,
		Location:    loc,
		Masked:      s.Address(),
		Confidence:  domain.Medium,
		Reasons:     []string{set.Why},
		TestPath:    IsTestPath(loc.Path),
		Fingerprint: fingerprint(c.Detector, loc.Path, s.Address(), 0),
		Store:       s,
	}
	switch {
	case set.State == domain.Unknown:
		f.Confidence = domain.Low
		f.Reasons = append(f.Reasons, "check this by hand")
	case s.PersonalData:
		f.Confidence = domain.High
	}
	if s.PersonalData {
		f.Reasons = append(f.Reasons, "linked to personal data: "+s.PersonalWhy)
	} else {
		f.Reasons = append(f.Reasons, "not linked to personal data by a tag or a table name")
	}
	if c.Detector == "tf-non-eea-region" && s.Adequacy != "" {
		f.Confidence = lower1(f.Confidence)
		f.Reasons = append(f.Reasons, fmt.Sprintf("%s has an EU adequacy decision, so the transfer is allowed but should be recorded", s.Adequacy))
	}
	if f.TestPath {
		f.Confidence = lower1(f.Confidence)
		f.Reasons = append(f.Reasons, "in a test, fixture or example path")
	}
	one := []domain.Finding{f}
	checked := map[int]bool{}
	for _, n := range []int{loc.Line, s.Location.Line} {
		if checked[n] || one[0].Ignored || n < 1 || n > len(text) {
			continue
		}
		checked[n] = true
		ctx := lineContext{line: text[n-1]}
		if n >= 2 {
			ctx.previous = text[n-2]
		}
		applyIgnore(one, ctx)
	}
	return one[0]
}
