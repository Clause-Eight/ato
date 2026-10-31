// Command bench measures ATO on a synthetic corpus: precision and recall per detector, against a
// pattern-only baseline that skips validation. Every value is generated here and belongs to nobody.
//
//	go run ./bench -lines 20000
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/engine"
	"github.com/Clause-Eight/ato/v2/internal/synth"
)

type line struct {
	text     string
	detector string
}

func main() {
	n := flag.Int("lines", 20000, "lines in the corpus")
	seed := flag.Int64("seed", 1, "random seed")
	export := flag.String("export", "", "also write the corpus and its labels to this folder (for bench/compare)")
	flag.Parse()
	corpus, body := corpusText(*seed, *n)
	if *export != "" {
		if err := writeCorpus(*export, body, corpus); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	start := time.Now()
	found, err := engine.ScanReader("corpus/data.txt", strings.NewReader(body), detectors.All())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	elapsed := time.Since(start)
	fmt.Printf("Scan time: %s (%.0f lines/s, one core)\n\n", elapsed.Round(time.Millisecond), float64(len(corpus))/elapsed.Seconds())
	fmt.Print(results(corpus, body, found, *seed))
}

// corpusText builds the corpus for a seed and joins it into one text.
func corpusText(seed int64, n int) ([]line, string) {
	corpus := build(rand.New(rand.NewSource(seed)), n)
	text := make([]string, len(corpus))
	for i, l := range corpus {
		text[i] = l.text
	}
	return corpus, strings.Join(text, "\n") + "\n"
}

// results is the corpus line and the per-detector table. It depends only on the seed and the detectors, so
// docs/BENCHMARKS.md can be checked against it in CI.
func results(corpus []line, body string, found []domain.Finding, seed int64) string {
	baseline, _ := engine.ScanReader("corpus/data.txt", strings.NewReader(body), patternOnly())
	// A regex scanner reports one name per match, so the baseline is scored without the possible identifiers.
	for i := range baseline {
		baseline[i].Possible = nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Corpus: %d lines (%d with a value, %d without), %.1f MB, seed %d\n\n", len(corpus), positives(corpus), len(corpus)-positives(corpus), float64(len(body))/1e6, seed)
	b.WriteString("| Detector | Values | Recall (medium) | Precision (medium) | False positives (medium) | Recall (low) | Precision (low) | Pattern-only precision |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, d := range detectors.All() {
		m := score(corpus, found, d.Name, domain.Medium)
		l := score(corpus, found, d.Name, domain.Low)
		p := score(corpus, baseline, d.Name, domain.Low)
		fmt.Fprintf(&b, "| `%s` | %d | %s | %s | %d | %s | %s | %s |\n", d.Name, m.expected, pct(m.recall()), pct(m.precision()), m.fp, pct(l.recall()), pct(l.precision()), pct(p.precision()))
	}
	all := score(corpus, found, "", domain.Medium)
	allLow := score(corpus, found, "", domain.Low)
	base := score(corpus, baseline, "", domain.Low)
	fmt.Fprintf(&b, "| **All** | %d | %s | %s | %d | %s | %s | %s |\n", all.expected, pct(all.recall()), pct(all.precision()), all.fp, pct(allLow.recall()), pct(allLow.precision()), pct(base.precision()))
	return b.String()
}

// writeCorpus writes corpus.txt and labels.tsv (line number, expected detector) for other tools to score.
func writeCorpus(dir, body string, corpus []line) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "corpus.txt"), []byte(body), 0o644); err != nil {
		return err
	}
	var b strings.Builder
	for i, l := range corpus {
		if l.detector != "" {
			fmt.Fprintf(&b, "%d\t%s\n", i+1, l.detector)
		}
	}
	return os.WriteFile(filepath.Join(dir, "labels.tsv"), []byte(b.String()), 0o644)
}

func positives(c []line) int {
	n := 0
	for _, l := range c {
		if l.detector != "" {
			n++
		}
	}
	return n
}

func patternOnly() []detectors.Detector {
	var out []detectors.Detector
	for _, d := range detectors.All() {
		d.Validate = nil
		d.Samples = nil
		out = append(out, d)
	}
	return out
}

type counts struct{ expected, tp, fp int }

func (c counts) recall() float64 {
	if c.expected == 0 {
		return -1
	}
	return float64(c.tp) / float64(c.expected)
}

func (c counts) precision() float64 {
	if c.tp+c.fp == 0 {
		return -1
	}
	return float64(c.tp) / float64(c.tp+c.fp)
}

func pct(v float64) string {
	if v < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

// score counts, for one detector (or all when name is ""), how many labelled lines were found by that
// detector at or above min, and how many findings landed on lines labelled otherwise. A finding that names
// several possible identifiers is right when one of them is the labelled one.
func score(corpus []line, found []domain.Finding, name string, min domain.Confidence) counts {
	var c counts
	hit := map[int]bool{}
	for _, f := range found {
		if f.Confidence < min || (name != "" && f.Detector != name && !possibly(f, name)) {
			continue
		}
		want := corpus[f.Location.Line-1].detector
		switch {
		case want != f.Detector && !possibly(f, want):
			c.fp++
		case (name == "" || want == name) && !hit[f.Location.Line]:
			c.tp++
			hit[f.Location.Line] = true
		}
	}
	for _, l := range corpus {
		if l.detector != "" && (name == "" || l.detector == name) {
			c.expected++
		}
	}
	return c
}

// possibly reports whether a finding names the detector among the identifiers the value could equally be.
func possibly(f domain.Finding, name string) bool {
	for _, p := range f.Possible {
		if p.Detector == name {
			return true
		}
	}
	return false
}

func build(r *rand.Rand, n int) []line {
	names := []string{}
	for _, d := range detectors.All() {
		names = append(names, d.Name)
	}
	sort.Strings(names)
	out := make([]line, 0, n)
	for i := 0; i < n; i++ {
		if r.Intn(3) == 0 {
			name := names[r.Intn(len(names))]
			out = append(out, line{text: withContext(r, name, valid(r, name)), detector: name})
			continue
		}
		out = append(out, line{text: noise(r)})
	}
	return out
}

// withContext writes a value under its keyword or a neutral name. A code that is personal data only in a record
// about someone, such as a diagnosis code, is written in a record with a patient key.
func withContext(r *rand.Rand, name, value string) string {
	d, _ := detectors.Get(name)
	record := "  "
	if d.NeedsPerson {
		record = fmt.Sprintf("  patient_id: %d, ", r.Intn(1000))
	}
	if r.Intn(2) == 0 && len(d.Keywords) > 0 {
		return fmt.Sprintf("%s%s: %q", record, d.Keywords[0], value)
	}
	return fmt.Sprintf("%svalue_%d = %q", record, r.Intn(1000), value)
}

func digits(r *rand.Rand, n int) string { return synth.Digits(r, n) }

// valid returns a generated value for a detector; internal/synth has a generator for every detector.
func valid(r *rand.Rand, name string) string {
	gen, ok := synth.For(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "no generator for %s\n", name)
		os.Exit(1)
	}
	return gen(r)
}

var words = []string{"order", "invoice", "build", "user", "session", "retry", "timeout", "version", "count", "id", "ref", "port"}

// noise is the kind of text that fills real repositories: IDs, timestamps in seconds and milliseconds, hashes,
// versions, amounts and minified JSON with several long numbers on one line.
func noise(r *rand.Rand) string {
	w := words[r.Intn(len(words))]
	switch r.Intn(11) {
	case 9:
		return fmt.Sprintf("  updated_at: %d", 1_600_000_000_000+r.Int63n(200_000_000_000))
	case 10:
		return fmt.Sprintf(`{"id":%d,"%s_id":%s,"ts":%d,"amount":%d.%02d}`, r.Intn(1_000_000), w, digits(r, 13), 1_600_000_000_000+r.Int63n(200_000_000_000), r.Intn(10000), r.Intn(100))
	case 0:
		return fmt.Sprintf("  %s_id: %s", w, digits(r, 9+r.Intn(8)))
	case 1:
		return fmt.Sprintf("  created_at: %d", 1_600_000_000+r.Intn(200_000_000))
	case 2:
		return fmt.Sprintf("  commit: %x", r.Uint64())
	case 3:
		return fmt.Sprintf("  version: %d.%d.%d", r.Intn(10), r.Intn(30), r.Intn(100))
	case 4:
		return fmt.Sprintf("  amount: %d.%02d", r.Intn(100000), r.Intn(100))
	case 5:
		return fmt.Sprintf("  %s = \"%s-%s\"", w, digits(r, 4), digits(r, 6))
	case 6:
		return fmt.Sprintf("  trace: %s%s", digits(r, 13), digits(r, 3))
	case 7:
		return fmt.Sprintf("  log.info(\"%s %d done\")", w, r.Intn(100000))
	}
	return fmt.Sprintf("  # %s handling for %s", w, words[r.Intn(len(words))])
}
