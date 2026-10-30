package fix

import (
	"math/rand"
	"strings"
	"testing"
	"unicode"

	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/domain"
	"github.com/Clause-Eight/ato/internal/synth"
)

func generated(t *testing.T, detector string, seed int64) string {
	t.Helper()
	g, ok := synth.For(detector)
	if !ok {
		t.Fatalf("no generator for %s", detector)
	}
	return g(rand.New(rand.NewSource(seed)))
}

func detector(t *testing.T, name string) detectors.Detector {
	t.Helper()
	d, ok := detectors.Get(name)
	if !ok {
		t.Fatalf("no detector %s", name)
	}
	return d
}

func TestReplaceIsStableValidAndMarked(t *testing.T) {
	for i, name := range []string{"ro-cnp", "iban", "it-fiscal-code"} {
		d := detector(t, name)
		v := generated(t, name, 1)
		a, ok := Replace(d, v)
		if !ok {
			t.Fatalf("%s: no replacement", name)
		}
		if i == 0 {
			if b, _ := Replace(d, v); a != b {
				t.Errorf("%s: two runs gave different replacements", name)
			}
		}
		if !a.Marked || !detectors.Synthetic(a.Text) {
			t.Errorf("%s: replacement is not marked", name)
		}
		if !fits(d, a.Text) || detectors.Canonical(a.Text) == detectors.Canonical(v) {
			t.Errorf("%s: replacement does not fit or equals the original", name)
		}
		if a.SameShape && len([]rune(a.Text)) != len([]rune(v)) {
			t.Errorf("%s: same shape but a different length", name)
		}
	}
}

func TestReplaceKeepsTheShapeWhereItCan(t *testing.T) {
	d := detector(t, "ro-cnp")
	v := generated(t, "ro-cnp", 5)
	r, _ := Replace(d, v)
	if !r.SameShape || len(r.Text) != len(v) {
		t.Fatalf("a CNP should keep its length")
	}
	d = detector(t, "iban")
	plain := generated(t, "iban", 9)
	plain = strings.ReplaceAll(plain, " ", "")
	var grouped []string
	for i := 0; i < len(plain); i += 4 {
		grouped = append(grouped, plain[i:min(i+4, len(plain))])
	}
	a, _ := Replace(d, plain)
	b, _ := Replace(d, strings.Join(grouped, " "))
	if detectors.Canonical(a.Text) != detectors.Canonical(b.Text) {
		t.Fatal("the same IBAN written with and without spaces should get the same replacement")
	}
}

func TestReplaceWithoutGenerator(t *testing.T) {
	if _, ok := Replace(detectors.Detector{Name: "no-such-detector"}, "123"); ok {
		t.Fatal("a detector without a generator cannot be fixed")
	}
}

func TestLayout(t *testing.T) {
	for _, c := range []struct{ orig, cand, want string }{
		{"AB12 34-5", "xy98765", "XY98 76-5"},
		{"ab-12", "CD34", "cd-34"},
		{"1234", "12345", ""},
		{"12345", "1234", ""},
	} {
		got, ok := layout(c.orig, c.cand, true)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("layout(%q, %q) = %q %v, want %q", c.orig, c.cand, got, ok, c.want)
		}
	}
}

func TestRunMasksTheDiffAndSkipsStaleFindings(t *testing.T) {
	cnp := generated(t, "ro-cnp", 21)
	card := generated(t, "card", 22)
	d := detector(t, "ro-cnp")
	line := `{"cnp": "` + cnp + `", "card": "` + card + `"}`
	start := strings.Index(line, cnp)
	cardStart := strings.Index(line, card)
	fixFinding := domain.Finding{Kind: domain.KindValue, Detector: "ro-cnp", Location: domain.Location{Path: "tests/u.json", Line: 2, Column: start + 1},
		Span: domain.Span{Start: start, End: start + len(cnp)}, Value: cnp, Masked: d.Mask(cnp)}
	cardFinding := domain.Finding{Kind: domain.KindValue, Detector: "card", Location: domain.Location{Path: "tests/u.json", Line: 2, Column: cardStart + 1},
		Span: domain.Span{Start: cardStart, End: cardStart + len(card)}, Value: card, Masked: strings.Repeat("•", len(card))}
	stale := fixFinding
	stale.Location.Line = 1
	content := "{}\r\n" + line + "\r\n"
	out := Run([]File{{Path: "tests/u.json", Content: []byte(content), Replace: []domain.Finding{fixFinding, stale}, All: []domain.Finding{fixFinding, cardFinding}}})
	o := out[0]
	if len(o.Changes) != 1 || o.Stale != 1 {
		t.Fatalf("changes %d stale %d", len(o.Changes), o.Stale)
	}
	if strings.Contains(o.Diff, cnp) || strings.Contains(o.Diff, card) {
		t.Fatal("the diff shows a found value")
	}
	if !strings.Contains(o.Diff, "+"+strings.Replace(strings.Replace(line, cnp, o.Changes[0].Replacement.Text, 1), card, cardFinding.Masked, 1)) {
		t.Fatalf("unexpected diff:\n%s", o.Diff)
	}
	got := string(o.Content)
	if strings.Contains(got, cnp) || !strings.Contains(got, card) || !strings.HasSuffix(got, "\"}\r\n") || !strings.HasPrefix(got, "{}\r\n") {
		t.Fatalf("unexpected content %q", got)
	}
}

// family is what a test on a value of the detector commonly checks besides its format: the country, brand, kind
// or class. Detectors without one return "".
func family(t *testing.T, name, text string) string {
	t.Helper()
	c := detectors.Canonical(text)
	switch name {
	case "iban", "eu-vat":
		return c[:2]
	case "card":
		return detectors.CardBrand(detectors.DigitsOnly(text))
	case "eu-phone":
		n := detector(t, name).Normalize(text)
		if strings.HasPrefix(n, "+") {
			return n[:4]
		}
		return n[:3]
	case "email":
		return strings.ToLower(text[strings.LastIndexByte(text, '.'):])
	case "ro-cnp":
		return c[:1]
	case "bg-egn":
		return string([]byte{'0' + (c[2]-'0')/2, '0' + (c[8]-'0')%2})
	case "ip-address":
		if i := strings.IndexByte(text, ':'); i >= 0 {
			return text[:i]
		}
		return "IPv4"
	case "mac-address":
		return text[:2]
	case "crypto-wallet":
		if l := strings.ToLower(text); strings.HasPrefix(l, "bc1") {
			return l[:4]
		}
		return strings.ToLower(text[:1])
	}
	return ""
}

// likeTemplates are shapes the generators rarely or never write: other countries, brands, separators and case.
var likeTemplates = map[string][]string{
	"iban":       {"BE00 0000 0000 0000", "DK0000000000000000", "MT00AAAA00000AAAAAAAAAAAAAAAAAA", "gb00 aaaa 0000 0000 0000 00", "RO00-AAAA-0000-0000-0000-0000"},
	"card":       {"3530 0000 0000 0000", "6011-0000-0000-0000", "4000000000000", "2221 0000 0000 0000", "3700 000000 00000", "6759000000000000"},
	"eu-phone":   {"+32 470 00 00 00", "0049 (0)30 1234567", "+353-87-000-0000", "0740 000 000", "+30 69 0000 0000"},
	"email":      {"jane.doe99@company.ro", "A.B@Firma.DE", "first_last+tag@mail.corp.co.uk"},
	"eu-vat":     {"NL000000000B01", "ATU00000000", "SE000000000001", "RO123456", "HU00000000", "FR00000000000", "IE0000000WA"},
	"ro-cnp":     {"7000101400000", "3000101400000"},
	"bg-egn":     {"0041010000", "0021010010"},
	"ip-address": {"93.184.216.34", "2a02:1234::1"},
}

// fewValues reports whether there are too few values with the letters and digits of v in the same positions,
// after the letters it starts with, for one to be likely to carry the mark (a four-digit postcode, the IPv4
// address 51.7.57.9): those may change format to keep the mark.
func fewValues(v string) bool {
	n := 1.0
	for i, r := range v {
		switch {
		case unicode.IsDigit(r):
			n *= 10
		case unicode.IsLetter(r) && strings.IndexFunc(v[:i], unicode.IsDigit) >= 0:
			n *= 20
		}
	}
	return n < 1<<22
}

// originals are values of every detector with a generator, and of the shapes in likeTemplates.
func originals(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, name := range synth.Names() {
		d, ok := detectors.Get(name)
		if !ok {
			continue
		}
		for s := int64(1); s <= 3; s++ {
			out[name] = append(out[name], generated(t, name, s))
		}
		for _, tmpl := range likeTemplates[name] {
			g, ok := synth.Like(name, tmpl)
			if !ok {
				t.Fatalf("%s: no shaper for template %d", name, len(out[name]))
			}
			r := rand.New(rand.NewSource(1))
			for i := 0; ; i++ {
				v, _ := layout(tmpl, g(r), true)
				if fits(d, v) {
					out[name] = append(out[name], v)
					break
				}
				if i > 1000 {
					t.Fatalf("%s: template %d gives no valid value", name, len(out[name]))
				}
			}
		}
	}
	return out
}

// TestReplaceKeepsCountryBrandAndFormat: a replacement keeps the length, separators, case and letter and digit
// positions of the original, and its country, brand or class. Values are never printed, only their index.
func TestReplaceKeepsCountryBrandAndFormat(t *testing.T) {
	// The case of a mixed-case Ethereum address is its checksum and Base58 addresses vary in length; a street
	// address is free text whose length follows the street name.
	loose := map[string]bool{"crypto-wallet": true, "street-address": true}
	for name, values := range originals(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := detector(t, name)
			if raceDetector && len(values) > 1 {
				values = values[:1] // the full set runs without -race; with it, the slowest searches exceed CI's limit
			}
			for i, v := range values {
				r, ok := Replace(d, v)
				if !ok {
					t.Errorf("value %d: no replacement", i)
					continue
				}
				if f, g := family(t, name, v), family(t, name, r.Text); f != g {
					t.Errorf("value %d: the replacement changed the country, brand or class", i)
				}
				if loose[name] || fewValues(v) {
					continue
				}
				a, b := []rune(v), []rune(r.Text)
				if !r.SameShape || len(a) != len(b) || classes(r.Text) != classes(v) {
					t.Errorf("value %d: the replacement changed the format", i)
					continue
				}
				for j := range a {
					sep := !isAlnum(a[j])
					if sep && a[j] != b[j] || !sep && unicode.IsUpper(a[j]) != unicode.IsUpper(b[j]) {
						t.Errorf("value %d: a separator or the case changed at %d", i, j)
						break
					}
				}
			}
		})
	}
}
