package infra

import "strings"

// block is a block of HCL: resource "aws_s3_bucket" "logs" { ... }. The file body is a block without a type.
type block struct {
	typ    string
	labels []string
	attrs  map[string]*attr
	blocks []*block
	line   int
}

type attr struct {
	name string
	val  *expr
	line int
}

type exprKind int

const (
	eOther exprKind = iota
	eLiteral
	eTemplate
	eRef
	eObject
	eTuple
	eCall
)

// expr is the part of an expression the reader understands: literals, string templates, references such as
// var.region or aws_kms_key.main.arn, objects, tuples and function calls. Anything else (operators,
// conditionals, for expressions) is eOther and is never resolved.
type expr struct {
	kind exprKind
	lit  string
	// str says a literal was written as a string, so "true" in quotes is still the string true.
	str   bool
	parts []tmpl
	ref   []string
	keys  []string
	items []*expr
	fn    string
	src   string
}

type tmpl struct {
	lit string
	e   *expr
}

// parse reads HCL source into its top-level body. Unparseable parts are skipped, never fatal.
func parse(src string) *block {
	p := parser{toks: lex(src, 1), src: src}
	return p.body(false)
}

type parser struct {
	toks []token
	pos  int
	src  string
}

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) take() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

func (p *parser) is(kind tokKind, text string) bool {
	t := p.peek()
	return t.kind == kind && (text == "" || t.text == text)
}

func (p *parser) skipNewlines() {
	for p.is(tNewline, "") {
		p.pos++
	}
}

func (p *parser) body(nested bool) *block {
	b := &block{attrs: map[string]*attr{}}
	for {
		p.skipNewlines()
		t := p.peek()
		if t.kind == tEOF || nested && t.kind == tPunct && t.text == "}" {
			return b
		}
		if t.kind != tIdent {
			p.skipLine()
			continue
		}
		p.take()
		if p.is(tPunct, "=") {
			p.take()
			b.attrs[t.text] = &attr{name: t.text, val: p.expr(false), line: t.line}
			continue
		}
		child := &block{typ: t.text, line: t.line}
		for p.peek().kind == tString || p.peek().kind == tIdent {
			l := p.take()
			if l.kind == tIdent {
				child.labels = append(child.labels, l.text)
			} else {
				child.labels = append(child.labels, literalText(l.parts))
			}
		}
		if !p.is(tPunct, "{") {
			p.skipLine()
			continue
		}
		p.take()
		inner := p.body(true)
		child.attrs, child.blocks = inner.attrs, inner.blocks
		if p.is(tPunct, "}") {
			p.take()
		}
		b.blocks = append(b.blocks, child)
	}
}

// skipItem skips the rest of a collection item up to its comma, consuming at least one token so a parse loop
// always moves on. A closing bracket of the collection is left in place.
func (p *parser) skipItem() {
	start := p.pos
	p.skipExpr(true)
	switch {
	case p.is(tPunct, ","):
		p.take()
	case p.pos == start && !p.is(tPunct, "}") && !p.is(tPunct, "]"):
		p.take()
	}
}

// skipLine skips to the end of the line, over any brackets opened on it, and always consumes a token.
func (p *parser) skipLine() {
	if p.is(tPunct, ")") || p.is(tPunct, "]") || p.is(tPunct, "}") {
		p.take()
		return
	}
	depth := 0
	for {
		t := p.peek()
		switch {
		case t.kind == tEOF:
			return
		case t.kind == tNewline && depth <= 0:
			return
		case t.kind == tPunct && strings.Contains("([{", t.text):
			depth++
		case t.kind == tPunct && strings.Contains(")]}", t.text):
			if depth == 0 {
				return
			}
			depth--
		}
		p.take()
	}
}

func literalText(parts []part) string {
	var b strings.Builder
	for _, pt := range parts {
		if !pt.isExpr {
			b.WriteString(pt.lit)
		}
	}
	return b.String()
}

// terminator reports whether the next token ends an expression. inside is true within parentheses, call
// arguments and tuples, where a line end is only white space.
func (p *parser) terminator(inside bool) bool {
	if inside {
		p.skipNewlines()
	}
	t := p.peek()
	switch t.kind {
	case tEOF, tNewline:
		return true
	case tPunct:
		return t.text == "," || t.text == "}" || t.text == "]" || t.text == ")"
	}
	return false
}

// expr reads one expression. When it is more than one understood primary, the rest is skipped and the
// expression is eOther.
func (p *parser) expr(inside bool) *expr {
	start := p.peek().start
	e := p.primary()
	if !p.terminator(inside) {
		p.skipExpr(inside)
		e = &expr{kind: eOther}
	}
	if p.pos > 0 {
		end := p.toks[p.pos-1].end
		if end > start {
			e.src = strings.TrimSpace(p.src[start:end])
		}
	}
	return e
}

func (p *parser) skipExpr(inside bool) {
	depth := 0
	for {
		t := p.peek()
		switch {
		case t.kind == tEOF:
			return
		case t.kind == tNewline && depth == 0 && !inside:
			return
		case t.kind == tPunct && strings.Contains("([{", t.text):
			depth++
		case t.kind == tPunct && strings.Contains(")]}", t.text):
			if depth == 0 {
				return
			}
			depth--
		case t.kind == tPunct && t.text == "," && depth == 0:
			return
		}
		p.take()
	}
}

func (p *parser) primary() *expr {
	t := p.peek()
	switch {
	case t.kind == tString:
		p.take()
		return templateExpr(t.parts)
	case t.kind == tNumber:
		p.take()
		return &expr{kind: eLiteral, lit: t.text}
	case t.kind == tPunct && t.text == "-" && p.toks[p.pos+1].kind == tNumber:
		p.take()
		return &expr{kind: eLiteral, lit: "-" + p.take().text}
	case t.kind == tIdent:
		switch t.text {
		case "true", "false", "null":
			p.take()
			return &expr{kind: eLiteral, lit: t.text}
		}
		return p.traversal()
	case t.kind == tPunct && t.text == "{":
		return p.object()
	case t.kind == tPunct && t.text == "[":
		return p.tuple()
	case t.kind == tPunct && t.text == "(":
		p.take()
		e := p.expr(true)
		if p.is(tPunct, ")") {
			p.take()
		}
		return e
	}
	return &expr{kind: eOther}
}

func (p *parser) traversal() *expr {
	name := p.take().text
	if p.is(tPunct, "(") {
		p.take()
		call := &expr{kind: eCall, fn: name}
		for {
			p.skipNewlines()
			if p.is(tPunct, ")") || p.is(tEOF, "") {
				p.take()
				return call
			}
			if p.is(tPunct, "]") || p.is(tPunct, "}") {
				return call
			}
			arg := p.expr(true)
			if p.is(tPunct, "...") {
				p.take()
				arg = &expr{kind: eOther}
			}
			call.items = append(call.items, arg)
			if p.is(tPunct, ",") {
				p.take()
			}
		}
	}
	e := &expr{kind: eRef, ref: []string{name}}
	for {
		switch {
		case p.is(tPunct, ".") && (p.toks[p.pos+1].kind == tIdent || p.toks[p.pos+1].kind == tNumber):
			p.take()
			e.ref = append(e.ref, p.take().text)
		case p.is(tPunct, ".") && p.toks[p.pos+1].text == "*":
			p.take()
			p.take()
			e.ref = append(e.ref, "*")
		case p.is(tPunct, "["):
			p.take()
			if p.is(tPunct, "*") {
				p.take()
				e.ref = append(e.ref, "*")
			} else {
				idx := p.expr(true)
				if idx.kind == eLiteral {
					e.ref = append(e.ref, idx.lit)
				} else {
					e.ref = append(e.ref, "?")
				}
			}
			if p.is(tPunct, "]") {
				p.take()
			}
		default:
			return e
		}
	}
}

func (p *parser) object() *expr {
	p.take()
	e := &expr{kind: eObject}
	for {
		p.skipNewlines()
		t := p.peek()
		if t.kind == tEOF || t.kind == tPunct && t.text == "}" {
			p.take()
			return e
		}
		if t.kind == tPunct && (t.text == "]" || t.text == ")") {
			return e
		}
		if t.kind == tIdent && t.text == "for" {
			p.skipToClose("{", "}")
			return &expr{kind: eOther}
		}
		key := ""
		switch {
		case t.kind == tIdent:
			key = p.take().text
		case t.kind == tString:
			key = literalText(p.take().parts)
		case t.kind == tPunct && t.text == "(":
			p.take()
			k := p.expr(true)
			if p.is(tPunct, ")") {
				p.take()
			}
			if k.kind == eLiteral {
				key = k.lit
			}
		default:
			p.skipItem()
			continue
		}
		if !p.is(tPunct, "=") && !p.is(tPunct, ":") {
			p.skipItem()
			continue
		}
		p.take()
		v := p.expr(false)
		if key != "" {
			e.keys = append(e.keys, key)
			e.items = append(e.items, v)
		}
		if p.is(tPunct, ",") {
			p.take()
		}
	}
}

func (p *parser) tuple() *expr {
	p.take()
	e := &expr{kind: eTuple}
	for {
		p.skipNewlines()
		t := p.peek()
		if t.kind == tEOF || t.kind == tPunct && t.text == "]" {
			p.take()
			return e
		}
		if t.kind == tPunct && (t.text == "}" || t.text == ")") {
			return e
		}
		if t.kind == tIdent && t.text == "for" {
			p.skipToClose("[", "]")
			return &expr{kind: eOther}
		}
		e.items = append(e.items, p.expr(true))
		if !p.is(tPunct, "]") {
			p.skipItem()
		}
	}
}

// skipToClose skips past the bracket that closes one already opened.
func (p *parser) skipToClose(open, close string) {
	depth := 1
	for {
		t := p.take()
		switch {
		case t.kind == tEOF:
			return
		case t.kind == tPunct && t.text == open:
			depth++
		case t.kind == tPunct && t.text == close:
			depth--
			if depth == 0 {
				return
			}
		}
	}
}

// templateExpr turns string parts into a literal, a single interpolation, or a template.
func templateExpr(parts []part) *expr {
	if len(parts) == 0 {
		return &expr{kind: eLiteral, str: true}
	}
	if len(parts) == 1 && !parts[0].isExpr {
		return &expr{kind: eLiteral, lit: parts[0].lit, str: true}
	}
	e := &expr{kind: eTemplate}
	for _, pt := range parts {
		if !pt.isExpr {
			e.parts = append(e.parts, tmpl{lit: pt.lit})
			continue
		}
		var inner *expr
		if strings.HasPrefix(pt.src, "%{") {
			inner = &expr{kind: eOther, src: pt.src}
		} else {
			sub := parser{toks: lex(pt.src, pt.line), src: pt.src}
			inner = sub.expr(true)
			if !sub.is(tEOF, "") {
				inner = &expr{kind: eOther, src: pt.src}
			}
		}
		e.parts = append(e.parts, tmpl{e: inner})
	}
	if len(e.parts) == 1 && e.parts[0].e != nil {
		return e.parts[0].e
	}
	return e
}
