package detectors

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Greek police identity card number (αριθμός δελτίου αστυνομικής ταυτότητας, ΑΔΤ): letters, a dash and six digits.
// Older cards carry one letter, any of the Greek alphabet; newer ones two letters that look the same in Greek and
// Latin (Α Β Ε Ζ Η Ι Κ Μ Ν Ο Ρ Τ Υ Χ), so they are written either way. There is no check digit. Source: Microsoft
// Purview, "Greece national ID card" (format only),
// https://learn.microsoft.com/en-us/purview/sit-defn-greece-national-id-card
//
// The dash is often left out or written as a space, so both are accepted. Greek look-alike letters are read as
// their Latin twins before the letters are checked.
func init() {
	Register(Detector{
		Name:         "gr-id-card",
		Label:        "Greek identity card number (ΑΔΤ)",
		Country:      "GR",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[A-ZΑ-Ω]{1,2}[- ]?[0-9]{6}`),
		Normalize:    normalizeGRIDCard,
		Validate:     validGRIDCard,
		Formatted:    regexp.MustCompile(`^[A-ZΑ-Ω]{1,2}-[0-9]{6}$`),
		Keywords:     []string{"αριθμός αστυνομικής ταυτότητας", "αρ. αστυνομικής ταυτότητας", "αστυνομική ταυτότητα", "αστυνομικής ταυτότητας", "greek id card", "greek identity card"},
		Concepts:     []Concept{IDCard},
		Base:         domain.Low,
		Validation:   "letters fit the old or new series and six digits follow; there is no control digit",
		NeedsKeyword: true,
		Needs:        Needs{Check: hasGRIDCardStart},
	})
}

// grLookAlike maps the Greek capitals that share their shape with a Latin one.
var grLookAlike = map[rune]rune{'Α': 'A', 'Β': 'B', 'Ε': 'E', 'Ζ': 'Z', 'Η': 'H', 'Ι': 'I', 'Κ': 'K', 'Μ': 'M', 'Ν': 'N', 'Ο': 'O', 'Ρ': 'P', 'Τ': 'T', 'Υ': 'Y', 'Χ': 'X'}

// grIDCardSeries are the letters of the two-letter series.
const grIDCardSeries = "ABEZHIKMNOPTYX"

func normalizeGRIDCard(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == ' ' {
			continue
		}
		if l, ok := grLookAlike[r]; ok {
			r = l
		}
		b.WriteRune(r)
	}
	return b.String()
}

func validGRIDCard(s string) bool {
	letters := strings.TrimRight(s, "0123456789")
	if len(s)-len(letters) != 6 {
		return false
	}
	switch utf8.RuneCountInString(letters) {
	case 1:
		r, _ := utf8.DecodeRuneInString(letters)
		return isGreekCapital(r) || strings.ContainsRune(grIDCardSeries, r)
	case 2:
		for _, r := range letters {
			if !strings.ContainsRune(grIDCardSeries, r) {
				return false
			}
		}
		return true
	}
	return false
}

func isGreekCapital(r rune) bool { return r >= 'Α' && r <= 'Ω' && r != 0x3A2 }

// hasGRIDCardStart looks for six digits with a capital letter (Latin or Greek) before them, after at most one dash
// or space.
func hasGRIDCardStart(line string) bool {
	for i := 1; i+6 <= len(line); i++ {
		if line[i-1] >= '0' && line[i-1] <= '9' || !grDigitsAt(line, i) {
			continue
		}
		j := i
		if line[j-1] == '-' || line[j-1] == ' ' {
			j--
		}
		if r, _ := utf8.DecodeLastRuneInString(line[:j]); r >= 'A' && r <= 'Z' || isGreekCapital(r) {
			return true
		}
	}
	return false
}

// grDigitsAt reports whether six digits start at i.
func grDigitsAt(line string, i int) bool {
	for k := i; k < i+6; k++ {
		if line[k] < '0' || line[k] > '9' {
			return false
		}
	}
	return true
}
