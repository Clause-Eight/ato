package engine

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// codeLike is the kind of text most scanned files hold: code, prose and the occasional number.
func codeLike(r *rand.Rand, n int) []string {
	shapes := []string{
		"func handle%d(w http.ResponseWriter, r *http.Request) {",
		"\tif err != nil { return fmt.Errorf(\"load user %%w\", err) }",
		"// The cache is refreshed every %d minutes.",
		"  timeout: %ds",
		"import { Button } from \"./components/button-%d\";",
		"SELECT id, created_at FROM orders WHERE status = 'paid' LIMIT %d;",
		"    \"version\": \"%d.4.1\",",
		"## Section %d",
		"  order_id: 4%09d",
	}
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(shapes[r.Intn(len(shapes))], r.Intn(1000))
	}
	return out
}

// TestPrefilterKeepsWorkFlat is the scan-time gate: as detectors are added, the share of detector patterns
// that run on an ordinary line must stay low. It counts work instead of timing it, so it is stable in CI.
func TestPrefilterKeepsWorkFlat(t *testing.T) {
	dets := detectors.All()
	lines := codeLike(rand.New(rand.NewSource(3)), 5000)
	runs := 0
	for _, l := range lines {
		runs += len(candidates(l, countDigits(l), dets))
	}
	share := float64(runs) / float64(len(lines)*len(dets))
	if share > 0.30 {
		t.Fatalf("%.0f%% of detector patterns run on an ordinary line, the budget is 30%%: give new detectors a Needs prefilter", share*100)
	}
}

func BenchmarkScanCodeLike(b *testing.B) {
	body := strings.Join(codeLike(rand.New(rand.NewSource(3)), 20000), "\n")
	dets := detectors.All()
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		if _, err := ScanReader("bench.go", strings.NewReader(body), dets); err != nil {
			b.Fatal(err)
		}
	}
}

var sixDigits = detectors.Detector{
	Name: "six", Label: "Six digits", Country: "XX", Pattern: regexp.MustCompile(`\d{3} ?\d{3}`),
	Normalize: detectors.DigitsOnly, Validate: func(s string) bool { return len(s) == 6 }, Mask: detectors.MaskAll,
	Base: domain.Low, Validation: "six digits", Formatted: regexp.MustCompile(`^\d{3} \d{3}$`),
	Needs: detectors.Needs{Digits: 6},
}

func TestUsualFormatRaisesConfidence(t *testing.T) {
	fs, _ := ScanReader("a.txt", strings.NewReader("a 123456 b 123 456\n"), []detectors.Detector{sixDigits})
	if len(fs) != 2 || fs[0].Confidence != domain.Low || fs[1].Confidence != domain.Medium {
		t.Fatalf("got %+v", fs)
	}
}

func TestPrefilterSkipsLinesThatCannotMatch(t *testing.T) {
	d := sixDigits
	d.Validate = func(string) bool { t.Fatal("pattern ran on a line without enough digits"); return false }
	if _, err := ScanReader("a.txt", strings.NewReader("only 12345 here\n"), []detectors.Detector{d}); err != nil {
		t.Fatal(err)
	}
	lit := sixDigits
	lit.Needs = detectors.Needs{AnyOf: []string{"<<"}}
	if fs, _ := ScanReader("a.txt", strings.NewReader("123456\n"), []detectors.Detector{lit}); len(fs) != 0 {
		t.Fatalf("a missing literal must skip the detector: %+v", fs)
	}
}

func TestValueValidForTwoDetectorsIsReportedOnce(t *testing.T) {
	other := sixDigits
	other.Name, other.Label, other.Keywords, other.OwnWords = "six-b", "Other six", []string{"bnum"}, []string{"bnum"}
	fs, _ := ScanReader("a.txt", strings.NewReader("bnum 123456\n"), []detectors.Detector{sixDigits, other})
	if len(fs) != 1 || fs[0].Detector != "six-b" {
		t.Fatalf("want one finding from the detector whose name is nearby, got %+v", fs)
	}
	if !strings.Contains(strings.Join(fs[0].Reasons, "; "), "also valid as Six digits") {
		t.Errorf("the collision is not explained: %v", fs[0].Reasons)
	}
}

func TestKeywordsIgnoreDiacriticsAndCase(t *testing.T) {
	d := sixDigits
	d.MatchWords = []string{detectors.Fold("adóazonosító")}
	fs, _ := ScanReader("a.txt", strings.NewReader("ADOAZONOSITO_JEL: 123456\n"), []detectors.Detector{d})
	if len(fs) != 1 || fs[0].Confidence != domain.Medium {
		t.Fatalf("got %+v", fs)
	}
}

func TestKeywordGatedDetectorNeedsAKeyword(t *testing.T) {
	d := sixDigits
	d.NeedsKeyword, d.MatchWords = true, []string{"serial"}
	if fs, _ := ScanReader("a.txt", strings.NewReader("123456\n"), []detectors.Detector{d}); len(fs) != 0 {
		t.Fatalf("reported without a keyword: %+v", fs)
	}
	if fs, _ := ScanReader("a.txt", strings.NewReader("serial:\n123456\n"), []detectors.Detector{d}); len(fs) != 1 {
		t.Fatalf("a keyword on the line before must allow it: %+v", fs)
	}
	d.KeywordOnLine = true
	if fs, _ := ScanReader("a.txt", strings.NewReader("serial: 123456\n123456\n"), []detectors.Detector{d}); len(fs) != 1 || fs[0].Location.Line != 1 {
		t.Fatalf("with KeywordOnLine only the value on the keyword's line counts: %+v", fs)
	}
}

func TestUnlikelyWordLowersConfidence(t *testing.T) {
	d := sixDigits
	d.Base, d.Formatted, d.MatchWords, d.Unlikely = domain.Medium, nil, []string{"pin"}, []string{"order"}
	fs, _ := ScanReader("a.txt", strings.NewReader("order: 123456\npin order 123456\n"), []detectors.Detector{d})
	if len(fs) != 2 || fs[0].Confidence != domain.Low || fs[1].Confidence != domain.High {
		t.Fatalf("got %+v", fs)
	}
}
