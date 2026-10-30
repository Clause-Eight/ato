package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/engine"
)

// TestBenchmarkTableIsUpToDate regenerates the per-detector precision and recall in docs/BENCHMARKS.md, so the
// published numbers always match the code. Run ATO_UPDATE_GOLDEN=1 go test ./bench after changing a detector.
func TestBenchmarkTableIsUpToDate(t *testing.T) {
	const seed, lines = 1, 30000
	corpus, body := corpusText(seed, lines)
	found, err := engine.ScanReader("corpus/data.txt", strings.NewReader(body), detectors.All())
	if err != nil {
		t.Fatal(err)
	}
	got := results(corpus, body, found, seed)
	path := filepath.Join("..", "docs", "BENCHMARKS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	const start, end = "<!-- bench:start -->\n", "<!-- bench:end -->"
	i, j := strings.Index(text, start), strings.Index(text, end)
	if i < 0 || j < i {
		t.Fatal("docs/BENCHMARKS.md has no bench markers")
	}
	want := text[:i+len(start)] + got + text[j:]
	if os.Getenv("ATO_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if text != want {
		t.Fatal("the benchmark table in docs/BENCHMARKS.md is out of date: run ATO_UPDATE_GOLDEN=1 go test ./bench")
	}
}
