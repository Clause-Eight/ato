package engine

import (
	"github.com/Clause-Eight/ato/v2/internal/art9"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// RecordDetectors are the names of the checks on records holding Art. 9 data: HL7 and FHIR patients, genotype
// files, biometric templates. Each can be switched off in settings like a detector.
func RecordDetectors() []string {
	out := make([]string, 0, len(art9.Checks))
	for _, c := range art9.Checks {
		out = append(out, c.Detector)
	}
	return out
}

// recordScanner adds what every finding needs to the art9 package's records: test paths, ignore comments and
// fingerprints. A fingerprint is built from the record's kind and its place among the file's records of that
// kind, never from the line, which holds the record's values.
type recordScanner struct {
	s        *art9.Scanner
	path     string
	testPath bool
	seen     map[string]int
}

func newRecordScanner(path string, enabled []string) *recordScanner {
	if len(enabled) == 0 {
		return nil
	}
	return &recordScanner{s: art9.NewScanner(path, enabled), path: path, testPath: IsTestPath(path), seen: map[string]int{}}
}

func (r *recordScanner) line(ctx lineContext) []domain.Finding {
	return r.findings(r.s.Line(ctx.number, ctx.line))
}

func (r *recordScanner) end() []domain.Finding {
	return r.findings(r.s.End())
}

func (r *recordScanner) findings(hits []art9.Hit) []domain.Finding {
	if len(hits) == 0 {
		return nil
	}
	out := make([]domain.Finding, 0, len(hits))
	for _, h := range hits {
		f := h.Finding
		f.TestPath = r.testPath
		if f.TestPath {
			// A record has no check digit, so a test path lowers it like the other detectors without one (ADR 10).
			f.Confidence = lower1(f.Confidence)
			f.Reasons = append(f.Reasons, "in a test, fixture or example path")
		}
		key := f.Detector + "\x00" + f.Masked
		f.Fingerprint = fingerprint(f.Detector, r.path, f.Masked, r.seen[key])
		r.seen[key]++
		one := []domain.Finding{f}
		applyIgnore(one, lineContext{line: h.Line, previous: h.Previous})
		out = append(out, one[0])
	}
	return out
}
