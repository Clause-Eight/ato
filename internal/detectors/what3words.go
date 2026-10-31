package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// what3words addresses name a 3 m square with three words joined by full stops, written after three slashes
// (///filled.count.soap) or in a link (w3w.co/..., what3words.com/...). Words are letters and combining marks in
// any script, and the stop may be the full-width or ideographic one
// (https://developer.what3words.com/tutorial/detecting-if-text-is-in-the-format-of-a-3-word-address). Three
// words alone are any dotted name in code, so the prefix or the link is required. A URL whose scheme ends in
// ":///" (file:///a.b.c, dns:///host.example.com) or a name with more than three parts is not an address; only
// the what3words API can say whether an address exists.
func init() {
	Register(Detector{
		Name:        "what3words",
		Label:       "what3words address",
		Country:     "EU",
		OnByDefault: true,
		Pattern: regexp.MustCompile(`(?:[A-Za-z][A-Za-z0-9+.-]*:)?/*///` + w3wWords + `(?:[.:][\p{L}\p{N}_-]+)*` +
			`|(?:https?://)?(?:www\.)?(?:w3w\.co|what3words\.com)/` + w3wWords),
		Validate:   validWhat3words,
		Mask:       maskProfile,
		Keywords:   []string{"what3words", "w3w", "3 word address", "three word address"},
		Concepts:   []Concept{Location},
		Base:       domain.Medium,
		Validation: "three words after /// or in a what3words link",
		Needs:      Needs{AnyOf: []string{"///", "w3w.co/", "what3words.com/"}},
	})
}

const (
	w3wWord  = `[\p{Ll}\p{Lo}\p{M}]+`
	w3wStop  = `[.｡。･・]`
	w3wWords = w3wWord + w3wStop + w3wWord + w3wStop + w3wWord
)

var w3wExact = regexp.MustCompile(`^` + w3wWords + `$`)

// w3wNotLast are endings of host and file names, which no address ends with.
var w3wNotLast = words("com net org io dev eu json yaml yml txt html go internal local svc cluster")

func validWhat3words(s string) bool {
	var rest string
	switch {
	case strings.HasPrefix(s, "///") && !strings.HasPrefix(s, "////"):
		rest = s[3:]
	default:
		s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
		s = strings.TrimPrefix(s, "www.")
		var ok bool
		if rest, ok = strings.CutPrefix(s, "w3w.co/"); !ok {
			if rest, ok = strings.CutPrefix(s, "what3words.com/"); !ok {
				return false
			}
		}
	}
	if !w3wExact.MatchString(rest) {
		return false
	}
	parts := strings.FieldsFunc(rest, func(r rune) bool { return strings.ContainsRune(".｡。･・", r) })
	return !w3wNotLast[parts[len(parts)-1]]
}
