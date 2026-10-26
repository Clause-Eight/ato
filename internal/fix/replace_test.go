package fix

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/synth"
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
		got, ok := layout(c.orig, c.cand)
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
