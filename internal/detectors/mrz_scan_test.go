package detectors_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/domain"
	"github.com/Clause-Eight/ato/internal/engine"
)

func check731(s string) string {
	sum := 0
	for i, c := range s {
		v := 0
		if c >= '0' && c <= '9' {
			v = int(c - '0')
		} else if c >= 'A' && c <= 'Z' {
			v = int(c-'A') + 10
		}
		sum += v * []int{7, 3, 1}[i%3]
	}
	return fmt.Sprint(sum % 10)
}

// madeUpPassport is a whole TD3 MRZ for a made-up holder of the fictional state Utopia.
func madeUpPassport() (upper, lower string) {
	upper = "P<UTOEXAMPLE<<TEST<PERSON" + strings.Repeat("<", 19)
	doc, birth, expiry := "QX7700412", "880517", "320704"
	doc, birth, expiry = doc+check731(doc), birth+check731(birth), expiry+check731(expiry)
	optional := strings.Repeat("<", 15)
	lower = doc + "UTO" + birth + "M" + expiry + optional + check731(doc+birth+expiry+optional)
	return upper, lower
}

func scanMRZ(t *testing.T, text string) []domain.Finding {
	t.Helper()
	d, ok := detectors.Get("mrz")
	if !ok {
		t.Fatal("mrz is not registered")
	}
	fs, err := engine.ScanReader("app/data.txt", strings.NewReader(text), []detectors.Detector{d})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestMRZScanReportsOnlyTheMaskedNumberLine(t *testing.T) {
	upper, lower := madeUpPassport()
	fs := scanMRZ(t, "machine readable zone:\n"+upper+"\n"+lower+"\n")
	if len(fs) != 1 || fs[0].Location.Line != 3 {
		t.Fatalf("want one finding on the number line, got %d", len(fs))
	}
	if fs[0].Masked != strings.Repeat("•", 44) || fs[0].Confidence < domain.Medium {
		t.Errorf("want a fully masked line at medium confidence or more, got confidence %v", fs[0].Confidence)
	}
	if strings.Contains(fs[0].Masked+strings.Join(fs[0].Reasons, " "), "EXAMPLE") {
		t.Error("the name appears in a finding")
	}
}

func TestMRZScanFindsTheLineInAStringConstant(t *testing.T) {
	upper, lower := madeUpPassport()
	fs := scanMRZ(t, `const mrz = "`+upper+`\n`+lower+`"`+"\n")
	if len(fs) != 1 || fs[0].Value != lower {
		t.Fatalf("want the number line after an escaped newline, got %d findings", len(fs))
	}
}
