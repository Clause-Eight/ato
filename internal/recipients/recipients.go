// Package recipients finds the third-party services a code base sends data to, from dependency files, imports
// and API host names. It lists recipients for a record of processing; it does not prove that personal data is
// sent to them.
package recipients

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Recipient is a service that commonly receives personal data.
type Recipient struct {
	Name     string
	Purpose  string
	EEA      bool
	Packages []string
	Hosts    []string
}

// Evidence says how a use was found.
const (
	FromDependency = "dependency"
	FromImport     = "import"
	FromHost       = "host"
)

var (
	byPackage = map[string]*Recipient{}
	hostRe    *regexp.Regexp
	byHost    = map[string]*Recipient{}
)

func init() {
	var hosts []string
	for i := range catalog {
		r := &catalog[i]
		for _, p := range r.Packages {
			byPackage[strings.ToLower(p)] = r
		}
		for _, h := range r.Hosts {
			byHost[h] = r
			hosts = append(hosts, regexp.QuoteMeta(h))
		}
	}
	sort.Slice(hosts, func(i, j int) bool { return len(hosts[i]) > len(hosts[j]) })
	hostRe = regexp.MustCompile(`(?i)(?:^|[^\w.-])((?:[\w-]+\.)*(?:` + strings.Join(hosts, "|") + `))\b`)
}

// Catalog returns the known recipients, sorted by name.
func Catalog() []Recipient {
	out := append([]Recipient(nil), catalog...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Candidate reports whether a file may name a recipient: a dependency file or source code.
func Candidate(p string) bool {
	if dependencyFile(path.Base(p)) {
		return true
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".go", ".py", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".java", ".kt", ".rb", ".php", ".cs", ".yml", ".yaml", ".env", ".json", ".toml", ".tf":
		return true
	}
	return false
}

// Find returns the recipients a file names, at most once per recipient, evidence and line.
func Find(p string, data []byte) []domain.RecipientUse {
	name := path.Base(p)
	var out []domain.RecipientUse
	seen := map[string]bool{}
	add := func(r *Recipient, evidence string, line, col int) {
		key := r.Name + "\x00" + evidence
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, domain.RecipientUse{
			Name: r.Name, Purpose: r.Purpose, EEA: r.EEA, Evidence: evidence,
			Location: domain.Location{Path: p, Line: line, Column: col},
		})
	}
	dep := dependencyFile(name)
	for i, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if dep {
			for _, tok := range dependencyNames(name, l) {
				if r, ok := byPackage[strings.ToLower(tok)]; ok {
					add(r, FromDependency, i+1, strings.Index(l, tok)+1)
				}
			}
		} else {
			for _, imp := range importNames(l) {
				if r := lookupImport(imp.name); r != nil {
					add(r, FromImport, i+1, imp.col)
					break
				}
			}
		}
		for _, m := range hostRe.FindAllStringSubmatchIndex(l, -1) {
			host := strings.ToLower(l[m[2]:m[3]])
			if r := lookupHost(host); r != nil {
				add(r, FromHost, i+1, m[2]+1)
			}
		}
	}
	return out
}

func lookupHost(host string) *Recipient {
	for h, r := range byHost {
		if host == h || strings.HasSuffix(host, "."+h) {
			return r
		}
	}
	return nil
}

// lookupImport matches an import path or a prefix of it: "stripe.checkout" and "@sentry/node/tracing" count.
func lookupImport(tok string) *Recipient {
	tok = strings.ToLower(tok)
	for {
		if r, ok := byPackage[tok]; ok {
			return r
		}
		i := strings.LastIndexAny(tok, "./")
		if i <= 0 {
			return nil
		}
		tok = tok[:i]
	}
}

func dependencyFile(name string) bool {
	switch name {
	case "package.json", "go.mod", "requirements.txt", "pyproject.toml", "Pipfile", "Gemfile", "composer.json", "pom.xml", "build.gradle", "build.gradle.kts", "setup.py", "setup.cfg":
		return true
	}
	return strings.HasPrefix(name, "requirements") && strings.HasSuffix(name, ".txt")
}

var (
	quoted     = regexp.MustCompile(`["']([@\w./:-]+)["']`)
	reqName    = regexp.MustCompile(`^\s*([A-Za-z0-9][\w.-]*)`)
	goModName  = regexp.MustCompile(`^\s*(?:require\s+)?([\w.-]+\.[\w.-]+/[\w./-]+)\s+v`)
	mavenGroup = regexp.MustCompile(`<groupId>([\w.-]+)</groupId>\s*<artifactId>([\w.-]+)</artifactId>`)
	gradleDep  = regexp.MustCompile(`["']([\w.-]+):([\w.-]+)(?::[\w.-]+)?["']`)
)

// dependencyNames returns the package names on one line of a dependency file.
func dependencyNames(file, line string) []string {
	switch file {
	case "go.mod":
		if m := goModName.FindStringSubmatch(line); m != nil {
			return goPrefixes(m[1])
		}
		return nil
	case "pom.xml":
		if m := mavenGroup.FindStringSubmatch(line); m != nil {
			return []string{m[1] + ":" + m[2]}
		}
		return nil
	case "build.gradle", "build.gradle.kts":
		if m := gradleDep.FindStringSubmatch(line); m != nil {
			return []string{m[1] + ":" + m[2]}
		}
		return nil
	}
	if strings.HasPrefix(file, "requirements") || file == "Pipfile" {
		if m := reqName.FindStringSubmatch(line); m != nil && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			return []string{m[1]}
		}
	}
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(line, -1) {
		name := m[1]
		if i := strings.IndexAny(name, "<>=~^!; "); i > 0 && !strings.HasPrefix(name, "@") {
			name = name[:i]
		}
		out = append(out, name)
	}
	if m := reqName.FindStringSubmatch(line); m != nil && file == "pyproject.toml" {
		out = append(out, m[1])
	}
	return out
}

// goPrefixes returns a Go module path and its shorter prefixes, so a module under a catalogued path matches.
func goPrefixes(mod string) []string {
	var out []string
	parts := strings.Split(mod, "/")
	for i := len(parts); i >= 3; i-- {
		out = append(out, strings.Join(parts[:i], "/"))
	}
	return out
}

var importPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:from\s+([\w.]+)\s+import|import\s+([\w.]+))`),
	regexp.MustCompile(`(?:require\(|from\s+|import\s+)["']([@\w./-]+)["']`),
	regexp.MustCompile(`^\s*(?:import\s+(?:\w+\s+)?)?"([\w.-]+\.[\w.-]+/[\w./-]+)"`),
	regexp.MustCompile(`^\s*import\s+(?:static\s+)?([\w.]+)\s*;?`),
	regexp.MustCompile(`^\s*require\s+["']([\w/-]+)["']`),
	regexp.MustCompile(`^\s*use\s+([\w\\]+)`),
	regexp.MustCompile(`^\s*using\s+([\w.]+)\s*;`),
}

type imported struct {
	name string
	col  int
}

// importNames returns the modules a line may import, with their columns. Several patterns can match one line
// (`import Stripe from "stripe"`), so every candidate is returned.
func importNames(line string) []imported {
	var out []imported
	for _, re := range importPatterns {
		m := re.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		for g := 2; g+1 < len(m); g += 2 {
			if m[g] >= 0 {
				out = append(out, imported{strings.ReplaceAll(line[m[g]:m[g+1]], `\`, "/"), m[g] + 1})
			}
		}
	}
	return out
}
