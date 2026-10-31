// Package detectors defines what a detector is and holds the registry. Each identifier lives in its own file.
package detectors

import (
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"sync"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Detector finds one kind of identifier. Pattern finds candidates; Validate confirms them with the
// identifier's own rules (check digits, dates, lengths). A detector without Validate reports every
// candidate at its base confidence. TrimGroups retries a failed candidate without its last
// space- or dash-separated group, for numbers often followed by more digits (an expiry date, a reference).
//
// Keywords are the identifier's own names (cnp, pesel); Concepts add the shared words for that kind of data in
// every EU language. Formatted matches a candidate written the way the identifier is usually printed (groups,
// a slash, a country prefix) and raises its confidence one step. Needs is a cheap test a line must pass before
// Pattern runs; Register derives the minimum digit count from Pattern when it is left at zero.
type Detector struct {
	Name        string
	Label       string
	Country     string
	OnByDefault bool
	Pattern     *regexp.Regexp
	Normalize   func(string) string
	Validate    func(string) bool
	Mask        func(string) string
	Keywords    []string
	Concepts    []Concept
	Base        domain.Confidence
	Samples     map[string]bool
	Validation  string
	TrimGroups  bool
	Formatted   *regexp.Regexp
	Needs       Needs
	// Unlikely are words that usually label other numbers (trace, order, invoice). With one on the value's own
	// line and no keyword, a match drops one confidence step. They are folded when the registry is read.
	Unlikely []string
	// Rival is another identifier with its own check that shares the detector's shape (a GTIN product code
	// for cards). A value that passes the rival's check with one of its words on the same line is dropped.
	Rival *Rival
	// NeedsKeyword drops a match unless a keyword is on the line or the line before, for identifiers with no
	// check digit that would otherwise match too much (format-only ID card numbers, MAC addresses).
	NeedsKeyword bool
	// AlsoIn lists other countries that issue the same identifier (the Czech and Slovak birth number).
	AlsoIn []string

	// OwnWords are the folded Keywords; MatchWords add the concept words, except words that are another
	// detector's own name. Both are set when the registry is read.
	OwnWords   []string
	MatchWords []string
}

// Rival describes an identifier that is not personal data but can pass a detector's check. Words are folded
// when the registry is read.
type Rival struct {
	Words []string
	Valid func(string) bool
}

// referenceWords label the long numbers of business records (orders, invoices, requests, traces), which share
// their length with some national numbers, also run together with id, no or number as in orderId. Each detector
// gets its own copy, because the registry folds it in place.
func referenceWords() []string {
	base := []string{"order", "invoice", "request", "trace", "transaction", "txn", "shipment", "tracking", "awb", "correlation", "session", "receipt"}
	ws := append([]string{}, base...)
	for _, w := range base {
		ws = append(ws, w+"id", w+"no", w+"number")
	}
	return ws
}

// Countries is Country followed by AlsoIn.
func (d Detector) Countries() []string { return append([]string{d.Country}, d.AlsoIn...) }

// Needs is a prefilter: a line with fewer digits, without any of the literals, or failing Check cannot match.
// Check is for patterns whose start is costly to search for, and must be much cheaper than the pattern.
type Needs struct {
	Digits int
	AnyOf  []string
	Check  func(line string) bool
}

// Admits reports whether a line with this many digits passes the prefilter.
func (n Needs) Admits(line string, digits int) bool {
	if digits < n.Digits {
		return false
	}
	if n.Check != nil && !n.Check(line) {
		return false
	}
	if len(n.AnyOf) == 0 {
		return true
	}
	for _, lit := range n.AnyOf {
		if strings.Contains(line, lit) {
			return true
		}
	}
	return false
}

var (
	registry = map[string]Detector{}
	resolved bool
	mu       sync.Mutex
)

// Register adds a detector. It panics on a duplicate name, which is a programming error.
func Register(d Detector) {
	mu.Lock()
	_, dup := registry[d.Name]
	mu.Unlock()
	if dup {
		panic("detectors: duplicate name " + d.Name)
	}
	if d.Normalize == nil {
		d.Normalize = func(s string) string { return s }
	}
	if d.Mask == nil {
		d.Mask = MaskAll
	}
	if d.Pattern != nil {
		d.Pattern = regexp.MustCompile(d.Pattern.String())
		d.Pattern.Longest()
		if d.Needs.Digits == 0 {
			d.Needs.Digits = MinDigits(d.Pattern)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	registry[d.Name] = d
	resolved = false
}

// resolveWords fills OwnWords and MatchWords once every detector is registered.
func resolveWords() {
	mu.Lock()
	defer mu.Unlock()
	if resolved {
		return
	}
	owner := map[string]string{}
	for name, d := range registry {
		for _, k := range d.Keywords {
			owner[Fold(k)] = name
		}
	}
	for name, d := range registry {
		d.OwnWords = nil
		for _, k := range d.Keywords {
			d.OwnWords = append(d.OwnWords, Fold(k))
		}
		ws := append([]string{}, d.OwnWords...)
		for _, c := range d.Concepts {
			for _, w := range ConceptWords(c) {
				if o, ok := owner[w]; !ok || o == name {
					ws = append(ws, w)
				}
			}
		}
		d.MatchWords = unique(ws)
		for i, w := range d.Unlikely {
			d.Unlikely[i] = Fold(w)
		}
		if d.Rival != nil {
			for i, w := range d.Rival.Words {
				d.Rival.Words[i] = Fold(w)
			}
		}
		registry[name] = d
	}
	resolved = true
}

// MinDigits is the fewest ASCII digits any match of re contains.
func MinDigits(re *regexp.Regexp) int {
	tree, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		return 0
	}
	return minDigits(tree.Simplify())
}

func minDigits(re *syntax.Regexp) int {
	switch re.Op {
	case syntax.OpLiteral:
		n := 0
		for _, r := range re.Rune {
			if r >= '0' && r <= '9' {
				n++
			}
		}
		return n
	case syntax.OpCharClass:
		for i := 0; i < len(re.Rune); i += 2 {
			if re.Rune[i] < '0' || re.Rune[i+1] > '9' {
				return 0
			}
		}
		return 1
	case syntax.OpCapture, syntax.OpPlus:
		return minDigits(re.Sub[0])
	case syntax.OpRepeat:
		return re.Min * minDigits(re.Sub[0])
	case syntax.OpConcat:
		n := 0
		for _, sub := range re.Sub {
			n += minDigits(sub)
		}
		return n
	case syntax.OpAlternate:
		least := -1
		for _, sub := range re.Sub {
			if n := minDigits(sub); least < 0 || n < least {
				least = n
			}
		}
		return max(least, 0)
	}
	return 0
}

// All returns every registered detector, sorted by name.
func All() []Detector {
	resolveWords()
	out := make([]Detector, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns a detector by name.
func Get(name string) (Detector, bool) {
	resolveWords()
	d, ok := registry[name]
	return d, ok
}

// DigitsOnly removes everything that is not 0-9.
func DigitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Digits converts a string of ASCII digits to ints; ok is false for any other character.
func Digits(s string) ([]int, bool) {
	out := make([]int, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return nil, false
		}
		out[i] = int(c - '0')
	}
	return out, true
}

// MaskAll hides the whole value and keeps only its length.
func MaskAll(v string) string {
	return strings.Repeat("•", len([]rune(v)))
}

// MaskKeepLast hides all but the last n characters.
func MaskKeepLast(n int) func(string) string {
	return func(v string) string {
		r := []rune(v)
		if len(r) <= n {
			return MaskAll(v)
		}
		return strings.Repeat("•", len(r)-n) + string(r[len(r)-n:])
	}
}

// ValidDate checks a calendar date, including leap years.
func ValidDate(year, month, day int) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}
	days := [13]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if month == 2 && (year%4 == 0 && (year%100 != 0 || year%400 == 0)) {
		return day <= 29
	}
	return day <= days[month]
}
