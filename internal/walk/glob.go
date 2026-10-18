package walk

import (
	"fmt"
	"regexp"
	"strings"
)

type glob struct {
	pattern string
	re      *regexp.Regexp
}

// compileGlob turns a gitignore-like pattern into a regexp. `**` crosses folders, `*` and `?` do not.
// A pattern without a slash matches at any depth; a trailing slash matches everything under a folder.
func compileGlob(pattern string) (glob, error) {
	p := strings.TrimPrefix(pattern, "./")
	if p == "" {
		return glob{}, fmt.Errorf("%w: empty pattern", ErrBadGlob)
	}
	if strings.HasSuffix(p, "/") {
		p += "**"
	}
	if strings.HasPrefix(p, "/") {
		p = strings.TrimPrefix(p, "/")
	} else if !strings.Contains(strings.TrimSuffix(p, "/**"), "/") {
		p = "**/" + p
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case strings.HasPrefix(p[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(p[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("(?:/.*)?$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return glob{}, fmt.Errorf("%w %q: %v", ErrBadGlob, pattern, err)
	}
	return glob{pattern: pattern, re: re}, nil
}

func compileAll(patterns []string) ([]glob, error) {
	out := make([]glob, 0, len(patterns))
	for _, p := range patterns {
		g, err := compileGlob(p)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func matchAny(globs []glob, rel string) bool {
	for _, g := range globs {
		if g.re.MatchString(rel) {
			return true
		}
	}
	return false
}
