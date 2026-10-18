// Command bench measures pii-eu on a synthetic corpus: precision and recall per detector, against a
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

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/engine"
	"github.com/Clause-Eight/pii-eu/internal/synth"
)

type line struct {
	text     string
	detector string
}

func main() {
	n := flag.Int("lines", 20000, "lines in the corpus")
	seed := flag.Int64("seed", 1, "random seed")
	export := flag.String("export", "", "also write the corpus and its labels to this folder (for bench/presidio_compare.py)")
	flag.Parse()
	r := rand.New(rand.NewSource(*seed))
	corpus := build(r, *n)
	text := make([]string, len(corpus))
	for i, l := range corpus {
		text[i] = l.text
	}
	body := strings.Join(text, "\n") + "\n"
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
	baseline, _ := engine.ScanReader("corpus/data.txt", strings.NewReader(body), patternOnly())

	fmt.Printf("Corpus: %d lines (%d with a value, %d without), %.1f MB, seed %d\n", len(corpus), positives(corpus), len(corpus)-positives(corpus), float64(len(body))/1e6, *seed)
	fmt.Printf("Scan time: %s (%.0f lines/s, one core)\n\n", elapsed.Round(time.Millisecond), float64(len(corpus))/elapsed.Seconds())
	fmt.Println("| Detector | Values | Recall (medium) | Precision (medium) | False positives (medium) | Recall (low) | Precision (low) | Pattern-only precision |")
	fmt.Println("|---|---|---|---|---|---|---|---|")
	for _, d := range detectors.All() {
		m := score(corpus, found, d.Name, domain.Medium)
		l := score(corpus, found, d.Name, domain.Low)
		b := score(corpus, baseline, d.Name, domain.Low)
		fmt.Printf("| `%s` | %d | %s | %s | %d | %s | %s | %s |\n", d.Name, m.expected, pct(m.recall()), pct(m.precision()), m.fp, pct(l.recall()), pct(l.precision()), pct(b.precision()))
	}
	all := score(corpus, found, "", domain.Medium)
	allLow := score(corpus, found, "", domain.Low)
	base := score(corpus, baseline, "", domain.Low)
	fmt.Printf("| **All** | %d | %s | %s | %d | %s | %s | %s |\n", all.expected, pct(all.recall()), pct(all.precision()), all.fp, pct(allLow.recall()), pct(allLow.precision()), pct(base.precision()))
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
// detector at or above min, and how many findings landed on lines labelled otherwise.
func score(corpus []line, found []domain.Finding, name string, min domain.Confidence) counts {
	var c counts
	hit := map[int]bool{}
	for _, f := range found {
		if f.Confidence < min || (name != "" && f.Detector != name) {
			continue
		}
		want := corpus[f.Location.Line-1].detector
		if want == f.Detector {
			if !hit[f.Location.Line] {
				c.tp++
				hit[f.Location.Line] = true
			}
		} else {
			c.fp++
		}
	}
	for _, l := range corpus {
		if l.detector != "" && (name == "" || l.detector == name) {
			c.expected++
		}
	}
	return c
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

func withContext(r *rand.Rand, name, value string) string {
	d, _ := detectors.Get(name)
	if r.Intn(2) == 0 && len(d.Keywords) > 0 {
		return fmt.Sprintf("  %s: %q", d.Keywords[0], value)
	}
	return fmt.Sprintf("  value_%d = %q", r.Intn(1000), value)
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

// noise is the kind of text that fills real repositories: IDs, timestamps, hashes, versions and amounts.
func noise(r *rand.Rand) string {
	w := words[r.Intn(len(words))]
	switch r.Intn(9) {
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
