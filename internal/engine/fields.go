package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/classify"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/schema"
)

// SpecialCategoryDetector names the findings for schema fields holding GDPR Art. 9 data. It can be switched
// off in settings like a detector.
const SpecialCategoryDetector = "special-category"

// specialLabels are the finding labels per Art. 9 category.
var specialLabels = map[domain.SpecialCategory]string{
	domain.SpecialHealth:       "Field holding health data (GDPR Art. 9)",
	domain.SpecialGenetic:      "Field holding genetic data (GDPR Art. 9)",
	domain.SpecialBiometric:    "Field holding biometric data (GDPR Art. 9)",
	domain.SpecialRacialEthnic: "Field holding racial or ethnic origin (GDPR Art. 9)",
	domain.SpecialPolitical:    "Field holding political opinions (GDPR Art. 9)",
	domain.SpecialReligious:    "Field holding religious or philosophical beliefs (GDPR Art. 9)",
	domain.SpecialTradeUnion:   "Field holding trade union membership (GDPR Art. 9)",
	domain.SpecialSexLife:      "Field holding sex life or sexual orientation (GDPR Art. 9)",
}

// inventory classifies the fields declared in one file; fields that hold no personal data are left out.
func inventory(path string, data []byte) []domain.InventoryEntry {
	return classifyFields(schema.Fields(path, data))
}

// classifyFields keeps the fields whose names say they hold personal data.
func classifyFields(fields []domain.Field) []domain.InventoryEntry {
	var out []domain.InventoryEntry
	for _, f := range fields {
		if c, ok := classify.Field(f.Name); ok {
			out = append(out, domain.InventoryEntry{Field: f, Classification: c})
		}
	}
	return out
}

// specialFindings turns special-category entries into findings. The line text is used for ignore comments.
func specialFindings(entries []domain.InventoryEntry, text []string) []domain.Finding {
	var out []domain.Finding
	for _, e := range entries {
		if e.Classification.Special == "" {
			continue
		}
		f := e.Field
		name := f.Name
		if f.Container != "" {
			name = f.Container + "." + f.Name
		}
		finding := domain.Finding{
			Kind:        domain.KindField,
			Category:    e.Classification.Category,
			Special:     e.Classification.Special,
			Detector:    SpecialCategoryDetector,
			Label:       specialLabels[e.Classification.Special],
			Country:     domain.CrossBorder,
			Location:    f.Location,
			Masked:      name,
			Confidence:  domain.Medium,
			Reasons:     []string{fmt.Sprintf("the name contains %q", e.Classification.Matched), declaredIn(f.Format)},
			TestPath:    IsTestPath(f.Location.Path),
			Fingerprint: fingerprint(SpecialCategoryDetector, f.Location.Path, name, 0),
		}
		if dataFormats[f.Format] {
			finding.ColumnName = f.Name
		}
		if finding.TestPath {
			finding.Confidence = lower1(finding.Confidence)
			finding.Reasons = append(finding.Reasons, "in a test, fixture or example path")
		}
		ctx := lineContext{}
		if n := f.Location.Line; n >= 1 && n <= len(text) {
			ctx.line = text[n-1]
			if n >= 2 {
				ctx.previous = text[n-2]
			}
		}
		one := []domain.Finding{finding}
		applyIgnore(one, ctx)
		out = append(out, one[0])
	}
	return out
}

var dataFormats = map[string]bool{"csv": true, "tsv": true, "jsonl": true}

func declaredIn(format string) string {
	switch format {
	case "csv", "tsv":
		return "a column of a " + strings.ToUpper(format) + " file"
	case "jsonl":
		return "a key in a JSON Lines file"
	}
	return "declared in a " + format + " schema or model"
}

func sortInventory(es []domain.InventoryEntry) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i].Field.Location, es[j].Field.Location
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}

func splitLines(data []byte) []string {
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
}
