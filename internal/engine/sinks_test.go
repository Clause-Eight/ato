package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/domain"
)

func scanSource(t *testing.T, path, code string) []domain.Finding {
	t.Helper()
	fs, err := ScanText(path, strings.NewReader(code), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestSinkFindings(t *testing.T) {
	code := "package app\n\nfunc h(u User) {\n\tlog.Printf(\"signup %s\", u.Email)\n}\n"
	fs := scanSource(t, "app/handler.go", code)
	if len(fs) != 1 {
		t.Fatalf("got %+v", fs)
	}
	f := fs[0]
	if f.Kind != domain.KindSink || f.Detector != SinkDetector || f.Sink != domain.SinkLog || f.Category != domain.CategoryContact ||
		f.Confidence != domain.Medium || f.Location != (domain.Location{Path: "app/handler.go", Line: 4, Column: 26}) || f.Masked != "u.Email" || f.Fingerprint == "" || f.Value != "" {
		t.Fatalf("unexpected finding %+v", f)
	}
	moved := scanSource(t, "app/handler.go", "package app\n\n\n\nfunc h(u User) {\n\tlog.Printf(\"signup %s\", u.Email)\n}\n")
	if moved[0].Fingerprint != f.Fingerprint {
		t.Fatal("the fingerprint should not depend on the line number")
	}

	inTest := scanSource(t, "app/handler_test.go", code)
	if len(inTest) != 1 || inTest[0].Confidence != domain.Low || !inTest[0].TestPath {
		t.Fatalf("a test path should lower the confidence: %+v", inTest)
	}

	ignored := scanSource(t, "app/handler.go", "// pii-eu:ignore the address is a synthetic support mailbox\nlog.Printf(\"%s\", u.Email)\n")
	if len(ignored) != 1 || !ignored[0].Ignored {
		t.Fatalf("an ignore comment should apply: %+v", ignored)
	}

	if fs, _ := ScanText("app/handler.go", strings.NewReader(code), nil, false); len(fs) != 0 {
		t.Fatal("sinks were reported while switched off")
	}
	if fs := scanSource(t, "notes.txt", "log.Printf(\"%s\", u.Email)\n"); len(fs) != 0 {
		t.Fatal("text files are not source code")
	}
}

func TestSyntheticValuesAreSamples(t *testing.T) {
	marked := ""
	for i := 0; i < 1_000_000 && marked == ""; i++ {
		if v := fmt.Sprintf("%06d7", i); detectors.Synthetic(v) {
			marked = v
		}
	}
	if marked == "" {
		t.Fatal("no marked seven-digit value")
	}
	fs := scan(t, "a.txt", "secret "+marked+"\n")
	if len(fs) != 1 || !fs[0].Sample || fs[0].Confidence != domain.Low {
		t.Fatalf("a marked value should be a low-confidence sample: %+v", fs)
	}
	if fs[0].Span != (domain.Span{Start: 7, End: 14}) {
		t.Fatalf("span %+v", fs[0].Span)
	}
}
