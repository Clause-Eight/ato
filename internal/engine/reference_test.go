package engine

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/synth"
)

// synthValue is a synthetic value for a detector, made by internal/synth.
func synthValue(t *testing.T, detector string, seed int64) string {
	t.Helper()
	g, ok := synth.For(detector)
	if !ok {
		t.Fatalf("no generator for %s", detector)
	}
	return g(rand.New(rand.NewSource(seed)))
}

// scanWith scans one line with one registered detector and returns its findings.
func scanWith(t *testing.T, detector, line string) []domain.Finding {
	t.Helper()
	d, ok := detectors.Get(detector)
	if !ok {
		t.Fatalf("%s is not registered", detector)
	}
	fs, err := ScanReader("app.log", strings.NewReader(line+"\n"), []detectors.Detector{d})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestReferenceWordsLowerThirteenDigitNationalNumbers(t *testing.T) {
	cases := []struct {
		name   string
		format string
		want   domain.Confidence
	}{
		{"bare value", "value %s", domain.Medium},
		{"order id", "order_id=%s status=paid", domain.Low},
		{"camel case order id", `{"orderId": "%s"}`, domain.Low},
		{"invoice number", "invoiceNumber: %s", domain.Low},
		{"request", "request %s took 12ms", domain.Low},
		{"trace", "trace=%s", domain.Low},
		{"transaction", "transaction_id %s", domain.Low},
		{"shipment", "shipment %s dispatched", domain.Low},
		{"own keyword wins", "order placed, cnp %s", domain.High},
	}
	for _, detector := range []string{"ro-cnp", "si-emso"} {
		for seed := int64(1); seed <= 20; seed++ {
			v := synthValue(t, detector, seed)
			for _, c := range cases {
				line := fmt.Sprintf(c.format, v)
				if detector == "si-emso" {
					line = strings.ReplaceAll(line, "cnp", "emso")
				}
				fs := scanWith(t, detector, line)
				if len(fs) != 1 {
					t.Fatalf("%s, %s, seed %d: want one finding, got %d", detector, c.name, seed, len(fs))
				}
				if fs[0].Confidence != c.want {
					t.Errorf("%s, %s, seed %d: confidence %s, want %s", detector, c.name, seed, fs[0].Confidence, c.want)
				}
			}
		}
	}
}

func TestReferenceWordOnTheLineBeforeDoesNotLower(t *testing.T) {
	d, _ := detectors.Get("ro-cnp")
	for seed := int64(1); seed <= 20; seed++ {
		v := synthValue(t, "ro-cnp", seed)
		fs, err := ScanReader("app.log", strings.NewReader("# order handling\nvalue = "+v+"\n"), []detectors.Detector{d})
		if err != nil {
			t.Fatal(err)
		}
		if len(fs) != 1 || fs[0].Confidence != domain.Medium {
			t.Fatalf("seed %d: a reference word on the line before must not lower the value: %d findings", seed, len(fs))
		}
	}
}
