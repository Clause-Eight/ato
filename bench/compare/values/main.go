// Command values prints generated, valid values for chosen detectors as JSON lines, for the comparison
// benchmark in bench/compare. Every value comes from internal/synth and belongs to nobody.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/synth"
)

func main() {
	names := flag.String("detectors", "", "comma-separated detector names")
	n := flag.Int("n", 100, "values per detector")
	seed := flag.Int64("seed", 1, "random seed")
	flag.Parse()
	r := rand.New(rand.NewSource(*seed))
	enc := json.NewEncoder(os.Stdout)
	for _, name := range strings.Split(*names, ",") {
		gen, ok := synth.For(name)
		d, known := detectors.Get(name)
		if !ok || !known {
			fmt.Fprintf(os.Stderr, "no generator for %s\n", name)
			os.Exit(2)
		}
		for i := 0; i < *n; i++ {
			_ = enc.Encode(map[string]any{"detector": name, "value": gen(r), "keywords": d.Keywords})
		}
	}
}
