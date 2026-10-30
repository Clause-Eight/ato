// Package infra reads data stores (databases, buckets, queues, log groups) out of Terraform files without
// running Terraform: no binary, no providers, no state. It understands literals, variables with defaults,
// auto-loaded .tfvars, locals and references between resources in the same folder; anything else is unknown,
// never guessed. See docs/adr/0004-terraform-stores.md.
package infra

import (
	"path"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// File is one parsed Terraform or auto-loaded variables file.
type File struct {
	Path string
	// Lines are the file's lines, kept so callers can read ignore comments next to a resource.
	Lines []string
	body  *block
	vars  bool
}

// Candidate reports whether a path is a Terraform file to read: a .tf file, terraform.tfvars or a .auto.tfvars
// file, outside the .terraform folder where Terraform keeps downloaded modules.
func Candidate(p string) bool {
	if strings.HasPrefix(p, ".terraform/") || strings.Contains(p, "/.terraform/") {
		return false
	}
	name := strings.ToLower(path.Base(p))
	return strings.HasSuffix(name, ".tf") || isAutoVars(name)
}

func isAutoVars(name string) bool {
	return name == "terraform.tfvars" || strings.HasSuffix(name, ".auto.tfvars")
}

// Parse reads one file. It never fails: parts it cannot read are skipped.
func Parse(p string, data []byte) *File {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return &File{Path: p, Lines: strings.Split(text, "\n"), body: parse(text), vars: isAutoVars(strings.ToLower(path.Base(p)))}
}

// Stores returns the data stores declared in the files, grouped into modules by folder, sorted by path and line.
// Calls to modules that are not read, and that look like they create a store, are listed as stores of type
// "module" with unknown settings.
func Stores(files []*File) []domain.Store {
	byDir := map[string][]*File{}
	for _, f := range files {
		if f == nil {
			continue
		}
		dir := path.Dir(f.Path)
		byDir[dir] = append(byDir[dir], f)
	}
	scanned := map[string]bool{}
	for dir, fs := range byDir {
		for _, f := range fs {
			if !f.vars {
				scanned[dir] = true
			}
		}
	}
	var out []domain.Store
	for _, fs := range byDir {
		sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
		m := newModule(fs)
		for _, r := range m.order {
			if sp, ok := specs[r.typ]; ok {
				out = append(out, build(m, r, sp))
			}
		}
		for _, call := range m.calls {
			if s, ok := moduleStore(m, call, scanned); ok {
				out = append(out, s)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Location, out[j].Location
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	return out
}

// Types lists the resource types read as data stores, by provider.
func Types() map[string][]string {
	out := map[string][]string{}
	for typ, sp := range specs {
		out[sp.provider] = append(out[sp.provider], typ)
	}
	for _, ts := range out {
		sort.Strings(ts)
	}
	return out
}
