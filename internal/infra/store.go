package infra

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// spec says how to read one resource type as a store.
type spec struct {
	provider string
	service  string
	// names are the attributes that hold the store's name in the cloud.
	names  []string
	region func(c *check) ([]string, domain.Setting)
	assess func(c *check)
}

var specs = map[string]spec{}

func register(typ string, sp spec) { specs[typ] = sp }

// check reads the settings of one resource into a store.
type check struct {
	m *module
	r *resource
	s *domain.Store
}

func build(m *module, r *resource, sp spec) domain.Store {
	s := domain.Store{
		Type: r.typ, Name: r.name, Provider: sp.provider, Service: sp.service,
		Location: domain.Location{Path: r.file.Path, Line: r.body.line, Column: 1},
	}
	c := &check{m: m, r: r, s: &s}
	s.Names = c.names(sp.names)
	s.Tags = c.tags()
	s.PersonalData, s.PersonalWhy = personalTags(s.Tags)
	regions, found := sp.region(c)
	s.Regions = regions
	s.InEEA, s.Adequacy = residencyOf(sp.provider, regions, found)
	sp.assess(c)
	return s
}

func (c *check) names(attrs []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(n string) {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, name := range attrs {
		if a := c.r.body.attrs[name]; a != nil {
			v := c.m.eval(a.val)
			if v.known && v.obj == nil && v.list == nil {
				add(v.s)
			} else {
				add(strings.Trim(v.hint, " -_./:"))
			}
		}
	}
	if c.s.Provider == "aws" {
		if n, ok := c.tags()["Name"]; ok {
			add(n)
		}
	}
	return out
}

// tags reads tags (AWS, Azure) or labels (Google Cloud), with the AWS provider's default_tags underneath.
func (c *check) tags() map[string]string {
	out := map[string]string{}
	if c.s.Provider == "aws" {
		if p := c.provider(); p != nil {
			for _, d := range blocksNamed(p, "default_tags") {
				if a := d.attrs["tags"]; a != nil {
					for k, v := range c.m.eval(a.val).literalMap() {
						out[k] = v
					}
				}
			}
		}
	}
	for _, name := range []string{"tags", "labels"} {
		if a := c.r.body.attrs[name]; a != nil {
			for k, v := range c.m.eval(a.val).literalMap() {
				out[k] = v
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// provider is the provider block the resource uses: the one named by its provider argument, or the default.
func (c *check) provider() *block {
	key := c.s.Provider
	if key == "google" {
		if p := c.m.providers[key]; p == nil && c.m.providers["google-beta"] != nil {
			key = "google-beta"
		}
	}
	if a := c.r.body.attrs["provider"]; a != nil && a.val.kind == eRef && len(a.val.ref) == 2 {
		key = a.val.ref[0] + "." + a.val.ref[1]
	}
	return c.m.providers[key]
}

func blocksNamed(b *block, name string) []*block {
	var out []*block
	for _, x := range b.blocks {
		if x.typ == name {
			out = append(out, x)
		}
	}
	return out
}

// dynamicNamed reports whether a dynamic block generates blocks of the name, so their content is unknown.
func dynamicNamed(b *block, name string) bool {
	for _, x := range b.blocks {
		if x.typ == "dynamic" && len(x.labels) == 1 && x.labels[0] == name {
			return true
		}
	}
	return false
}

// lookup walks a dotted path of nested blocks to an attribute, through the first block of each name. dynamic
// is true when a dynamic block stands where a block on the path is missing.
func lookup(b *block, path string) (a *attr, dynamic bool) {
	steps := strings.Split(path, ".")
	for _, step := range steps[:len(steps)-1] {
		inner := blocksNamed(b, step)
		if len(inner) == 0 {
			return nil, dynamicNamed(b, step)
		}
		b = inner[0]
	}
	return b.attrs[steps[len(steps)-1]], false
}

// on keeps a line only when the resource is in the file of the store being checked.
func (c *check) on(r *resource, line int) int {
	if r.file != c.r.file {
		return 0
	}
	return line
}

func unresolved(path string, a *attr) string {
	src := a.val.src
	if src == "" {
		src = "an expression"
	}
	return fmt.Sprintf("%s is set from %s, which cannot be read without running Terraform", path, src)
}

func setting(state domain.Tri, why string) domain.Setting {
	return domain.Setting{State: state, Why: why}
}

// flag reads a boolean attribute: onTrue or onFalse when it is written, absent when it is not set.
func (c *check) flag(r *resource, path string, onTrue, onFalse domain.Tri, absent domain.Setting) domain.Setting {
	a, dynamic := lookup(r.body, path)
	if a == nil {
		if dynamic {
			return setting(domain.Unknown, fmt.Sprintf("%s comes from a dynamic block", path))
		}
		return absent
	}
	line := c.on(r, a.line)
	v := c.m.eval(a.val)
	switch {
	case v.known && v.s == "true":
		return domain.Setting{State: onTrue, Why: c.where(r, path+" = true"), Line: line}
	case v.known && v.s == "false":
		return domain.Setting{State: onFalse, Why: c.where(r, path+" = false"), Line: line}
	}
	return domain.Setting{State: domain.Unknown, Why: c.where(r, unresolved(path, a)), Line: line}
}

// number reads a numeric attribute: Yes when it is above zero, No when it is zero.
func (c *check) number(r *resource, path, unit string, absent domain.Setting) domain.Setting {
	a, dynamic := lookup(r.body, path)
	if a == nil {
		if dynamic {
			return setting(domain.Unknown, fmt.Sprintf("%s comes from a dynamic block", path))
		}
		return absent
	}
	line := c.on(r, a.line)
	v := c.m.eval(a.val)
	n, err := strconv.ParseFloat(v.s, 64)
	if !v.known || err != nil {
		return domain.Setting{State: domain.Unknown, Why: c.where(r, unresolved(path, a)), Line: line}
	}
	why := fmt.Sprintf("%s = %s", path, v.s)
	if unit != "" {
		why += " " + unit
	}
	if n > 0 {
		return domain.Setting{State: domain.Yes, Why: c.where(r, why), Line: line}
	}
	return domain.Setting{State: domain.No, Why: c.where(r, why), Line: line}
}

// text reads a string attribute; ok is false when it is set but cannot be read.
func (c *check) text(r *resource, path string) (s string, a *attr, ok bool) {
	a, _ = lookup(r.body, path)
	if a == nil {
		return "", nil, true
	}
	v := c.m.eval(a.val)
	if !v.known || v.obj != nil || v.list != nil {
		return "", a, false
	}
	return v.s, a, true
}

// where names another resource in a reason, when the setting comes from it.
func (c *check) where(r *resource, why string) string {
	if r == c.r {
		return why
	}
	return why + " in " + r.address()
}

// related returns resources of a type that configure this store through one of attrs, by reference or by the
// store's literal name, such as an aws_s3_bucket_versioning for a bucket.
func (c *check) related(typ string, attrs ...string) []*resource {
	var out []*resource
	for _, r := range c.m.order {
		if r.typ != typ {
			continue
		}
		for _, name := range attrs {
			a := r.body.attrs[name]
			if a == nil {
				continue
			}
			if c.m.target(a.val) == c.r {
				out = append(out, r)
				break
			}
			if v := c.m.eval(a.val); v.known && contains(c.s.Names, v.s) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// regionAttr reads the region from an attribute of the resource; fallback says what to do when it is not set.
func (c *check) regionAttr(name string, fallback func() ([]string, domain.Setting)) ([]string, domain.Setting) {
	a := c.r.body.attrs[name]
	if a == nil {
		if fallback != nil {
			return fallback()
		}
		return nil, setting(domain.Unknown, name+" is not set")
	}
	return c.regionFrom(c.r, a, name)
}

func (c *check) regionFrom(r *resource, a *attr, name string) ([]string, domain.Setting) {
	v := c.m.eval(a.val)
	regions, ok := v.strings()
	if !ok || len(regions) == 0 {
		return nil, domain.Setting{State: domain.Unknown, Why: c.where(r, unresolved(name, a)), Line: c.on(r, a.line)}
	}
	return regions, domain.Setting{Line: c.on(r, a.line)}
}

// providerRegion reads the region of the provider block the resource uses.
func (c *check) providerRegion() ([]string, domain.Setting) {
	p := c.provider()
	if p == nil {
		return nil, setting(domain.Unknown, fmt.Sprintf("no %s provider block in this folder sets the region", c.s.Provider))
	}
	a := p.attrs["region"]
	if a == nil {
		return nil, setting(domain.Unknown, fmt.Sprintf("the %s provider block sets no region, so it comes from the environment", c.s.Provider))
	}
	v := c.m.eval(a.val)
	if !v.known || v.obj != nil || v.list != nil {
		return nil, setting(domain.Unknown, "in the provider block, "+unresolved("region", a))
	}
	return []string{v.s}, setting(domain.NotApplicable, "from the "+c.s.Provider+" provider block")
}

// personalKeys are tag keys that mark personal data with a yes/no value, written without separators.
var personalKeys = map[string]bool{"pii": true, "personaldata": true, "containspii": true, "haspii": true, "gdpr": true, "personal": true, "containspersonaldata": true}

// classificationKeys are tag keys that hold a data classification.
var classificationKeys = map[string]bool{"dataclassification": true, "classification": true, "dataclass": true, "datasensitivity": true, "sensitivity": true, "datacategory": true, "datatype": true}

// personalWords in a classification value mean personal data.
var personalWords = []string{"pii", "personal", "gdpr", "phi", "health", "special"}

// personalTags reports whether a store's tags say it holds personal data.
func personalTags(tags map[string]string) (bool, string) {
	for _, k := range sortedKeys(tags) {
		key := strings.NewReplacer("_", "", "-", "", " ", "", ":", "", ".", "").Replace(strings.ToLower(k))
		val := strings.ToLower(strings.TrimSpace(tags[k]))
		if personalKeys[key] && (val == "true" || val == "yes" || val == "1" || val == "y") {
			return true, fmt.Sprintf("tagged %s = %s", k, val)
		}
		if classificationKeys[key] {
			for _, w := range personalWords {
				if strings.Contains(val, w) {
					return true, fmt.Sprintf("tagged %s with %q", k, w)
				}
			}
		}
	}
	return false, ""
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
