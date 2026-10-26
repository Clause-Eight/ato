// Package archtest checks that packages only depend in the allowed direction.
package archtest

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/Clause-Eight/pii-eu/internal/"

var allowed = map[string][]string{
	"domain":    {},
	"detectors": {"domain", "textfold"},
	"textfold":  {},
	"config":    {"domain"},
	"walk":      {"domain"},
	"engine":    {"domain", "detectors", "walk", "schema", "classify", "infra"},
	"schema":    {"domain"},
	"infra":     {"domain"},
	"report":    {"domain"},
	"baseline":  {"domain"},
	"synth":     {},
	"classify":  {"domain", "detectors", "textfold"},
}

func TestLayers(t *testing.T) {
	out, err := exec.Command("go", "list", "-json", "../...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var pkg struct {
			ImportPath string
			Imports    []string
		}
		if err := dec.Decode(&pkg); err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(pkg.ImportPath, module)
		rules, ok := allowed[strings.Split(name, "/")[0]]
		if !ok {
			continue
		}
		for _, imp := range pkg.Imports {
			if !strings.HasPrefix(imp, module) {
				continue
			}
			dep := strings.Split(strings.TrimPrefix(imp, module), "/")[0]
			if !contains(rules, dep) {
				t.Errorf("%s must not import %s", name, dep)
			}
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
