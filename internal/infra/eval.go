package infra

import (
	"sort"
	"strings"
)

// module is one folder of Terraform files: Terraform reads every .tf file in a folder as one module.
type module struct {
	vars      map[string]*expr
	tfvars    map[string]*expr
	locals    map[string]*expr
	resources map[string]*resource
	order     []*resource
	providers map[string]*block
}

type resource struct {
	typ, name string
	body      *block
	file      *File
}

func (r *resource) address() string { return r.typ + "." + r.name }

func newModule(files []*File) *module {
	m := &module{vars: map[string]*expr{}, tfvars: map[string]*expr{}, locals: map[string]*expr{}, resources: map[string]*resource{}, providers: map[string]*block{}}
	for _, f := range files {
		if f.vars {
			for name, a := range f.body.attrs {
				m.tfvars[name] = a.val
			}
			continue
		}
		for _, b := range f.body.blocks {
			switch {
			case b.typ == "variable" && len(b.labels) == 1:
				if a := b.attrs["default"]; a != nil {
					m.vars[b.labels[0]] = a.val
				}
			case b.typ == "locals":
				for name, a := range b.attrs {
					m.locals[name] = a.val
				}
			case b.typ == "provider" && len(b.labels) == 1:
				key := b.labels[0]
				if a := b.attrs["alias"]; a != nil {
					if v := m.eval(a.val); v.known {
						key += "." + v.s
					}
				}
				m.providers[key] = b
			case b.typ == "resource" && len(b.labels) == 2:
				r := &resource{typ: b.labels[0], name: b.labels[1], body: b, file: f}
				m.resources[r.address()] = r
				m.order = append(m.order, r)
			}
		}
	}
	return m
}

// value is what an expression resolves to. known is false when any part depends on something that cannot be
// read without running Terraform; hint then keeps the literal text of a partly known string, and obj the keys
// of a partly known object.
type value struct {
	known bool
	s     string
	hint  string
	obj   map[string]value
	list  []value
}

const maxDepth = 12

func (m *module) eval(e *expr) value { return m.evalAt(e, 0) }

func (m *module) evalAt(e *expr, depth int) value {
	if e == nil || depth > maxDepth {
		return value{}
	}
	switch e.kind {
	case eLiteral:
		if e.lit == "null" && !e.str {
			return value{}
		}
		return value{known: true, s: e.lit}
	case eTemplate:
		var s, hint strings.Builder
		known := true
		for _, p := range e.parts {
			if p.e == nil {
				s.WriteString(p.lit)
				hint.WriteString(p.lit)
				continue
			}
			v := m.evalAt(p.e, depth+1)
			if v.known && v.obj == nil && v.list == nil {
				s.WriteString(v.s)
				hint.WriteString(v.s)
				continue
			}
			known = false
			hint.WriteString(" " + v.hint + " ")
		}
		if known {
			return value{known: true, s: s.String()}
		}
		return value{hint: strings.Join(strings.Fields(hint.String()), " ")}
	case eRef:
		return m.ref(e.ref, depth)
	case eObject:
		v := value{known: true, obj: map[string]value{}}
		for i, k := range e.keys {
			item := m.evalAt(e.items[i], depth+1)
			v.obj[k] = item
		}
		return v
	case eTuple:
		v := value{known: true, list: []value{}}
		for _, it := range e.items {
			item := m.evalAt(it, depth+1)
			v.known = v.known && item.known
			v.list = append(v.list, item)
		}
		return v
	case eCall:
		return m.call(e, depth)
	}
	return value{}
}

func (m *module) call(e *expr, depth int) value {
	args := make([]value, len(e.items))
	for i, a := range e.items {
		args[i] = m.evalAt(a, depth+1)
	}
	switch e.fn {
	case "merge":
		out := value{known: true, obj: map[string]value{}}
		for _, a := range args {
			if a.obj == nil {
				out.known = false
				continue
			}
			out.known = out.known && a.known
			for k, v := range a.obj {
				out.obj[k] = v
			}
		}
		return out
	case "lower", "upper", "tostring", "trimspace":
		if len(args) != 1 || !args[0].known || args[0].obj != nil {
			return value{}
		}
		s := args[0].s
		switch e.fn {
		case "lower":
			s = strings.ToLower(s)
		case "upper":
			s = strings.ToUpper(s)
		case "trimspace":
			s = strings.TrimSpace(s)
		}
		return value{known: true, s: s}
	case "tobool", "tonumber":
		if len(args) == 1 {
			return args[0]
		}
	}
	return value{}
}

// ref resolves var.x, local.x and type.name.attribute; other references (data sources, module outputs, count,
// each) are not resolved.
func (m *module) ref(steps []string, depth int) value {
	if len(steps) < 2 {
		return value{}
	}
	var v value
	rest := steps[2:]
	switch steps[0] {
	case "var":
		e := m.tfvars[steps[1]]
		if e == nil {
			e = m.vars[steps[1]]
		}
		v = m.evalAt(e, depth+1)
	case "local":
		v = m.evalAt(m.locals[steps[1]], depth+1)
	case "data", "module", "each", "count", "self", "path", "terraform":
		return value{}
	default:
		r := m.resources[steps[0]+"."+steps[1]]
		if r == nil || len(rest) == 0 {
			return value{}
		}
		a := r.body.attrs[rest[0]]
		if a == nil {
			return value{}
		}
		v = m.evalAt(a.val, depth+1)
		rest = rest[1:]
	}
	for _, step := range rest {
		if v.obj == nil {
			return value{}
		}
		v = v.obj[step]
	}
	return v
}

// target returns the resource an expression refers to, such as aws_s3_bucket.logs for aws_s3_bucket.logs.id.
func (m *module) target(e *expr) *resource {
	if e == nil {
		return nil
	}
	if e.kind == eTemplate {
		for _, p := range e.parts {
			if p.e != nil {
				if r := m.target(p.e); r != nil {
					return r
				}
			}
		}
		return nil
	}
	if e.kind != eRef || len(e.ref) < 2 {
		return nil
	}
	return m.resources[e.ref[0]+"."+e.ref[1]]
}

// strings resolves an expression to a list of strings; known is false when any item cannot be read.
func (v value) strings() ([]string, bool) {
	if v.list == nil {
		if v.known && v.obj == nil {
			return []string{v.s}, true
		}
		return nil, false
	}
	var out []string
	for _, it := range v.list {
		if !it.known || it.obj != nil || it.list != nil {
			return out, false
		}
		out = append(out, it.s)
	}
	return out, v.known
}

// literalMap returns the keys of an object whose values are known strings, sorted for stable output.
func (v value) literalMap() map[string]string {
	out := map[string]string{}
	for k, x := range v.obj {
		if x.known && x.obj == nil && x.list == nil {
			out[k] = x.s
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
