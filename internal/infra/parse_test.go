package infra

import (
	"strings"
	"testing"
)

func TestParseSyntax(t *testing.T) {
	src := `# a comment
// another
/* a block
   comment */
terraform {
  required_version = ">= 1.5"
}

resource "aws_s3_bucket" "b" {
  bucket = "a-${var.env}-b" # trailing comment
  policy = <<-EOT
    {"Version": "2012-10-17", "Note": "${var.note}"}
  EOT
  single { enabled = true }
  list   = ["x", "y",
    "z"]
  obj = {
    "quoted key" = 1
    plain        = "two",
    nested       = { inner = true }
  }
  call = merge(
    var.a,
    { k = "v" },
  )
  cond     = var.x ? "a" : "b"
  for_list = [for s in var.list : upper(s)]
  for_obj  = { for k, v in var.map : k => v }
  escaped  = "say \"hi\" and $${literal}"
  negative = -3
  index    = var.list[0]
  splat    = aws_instance.web[*].id
}

resource "aws_dynamodb_table" "after" {
  name = "still read"
}
`
	body := parse(src)
	if len(body.blocks) != 3 {
		t.Fatalf("blocks: %d", len(body.blocks))
	}
	b := body.blocks[1]
	if b.typ != "resource" || strings.Join(b.labels, ",") != "aws_s3_bucket,b" || b.line != 9 {
		t.Fatalf("resource block: %+v", b)
	}
	if got := body.blocks[2].attrs["name"]; got == nil || got.val.lit != "still read" || got.line != 36 {
		t.Fatalf("parsing went off track before the last block: %+v", got)
	}
	kinds := map[string]exprKind{
		"bucket": eTemplate, "policy": eTemplate, "list": eTuple, "obj": eObject, "call": eCall,
		"cond": eOther, "for_list": eOther, "for_obj": eOther, "escaped": eLiteral, "negative": eLiteral, "index": eRef, "splat": eRef,
	}
	for name, kind := range kinds {
		a := b.attrs[name]
		if a == nil {
			t.Errorf("%s missing", name)
			continue
		}
		if a.val.kind != kind {
			t.Errorf("%s: kind %d, want %d (%q)", name, a.val.kind, kind, a.val.src)
		}
	}
	if got := b.attrs["escaped"].val.lit; got != `say "hi" and ${literal}` {
		t.Errorf("escapes: %q", got)
	}
	if got := b.attrs["cond"].val.src; got != `var.x ? "a" : "b"` {
		t.Errorf("source of an expression: %q", got)
	}
	obj := b.attrs["obj"].val
	if strings.Join(obj.keys, ",") != "quoted key,plain,nested" {
		t.Errorf("object keys: %v", obj.keys)
	}
	single := blocksNamed(b, "single")
	if len(single) != 1 || single[0].attrs["enabled"] == nil {
		t.Errorf("single-line block: %+v", single)
	}
	if got := b.attrs["list"].val.items; len(got) != 3 {
		t.Errorf("multi-line tuple: %d items", len(got))
	}
}

func TestEvalTemplatesAndFunctions(t *testing.T) {
	m := newModule([]*File{Parse("m/main.tf", []byte(`variable "env" {
  default = "prod"
}
variable "cfg" {
  default = {
    region = "eu-north-1"
  }
}
locals {
  name   = "${var.env}-orders"
  region = var.cfg.region
  upper  = upper(local.name)
  loop   = local.loop
}`))})
	cases := map[string]string{"name": "prod-orders", "region": "eu-north-1", "upper": "PROD-ORDERS"}
	for name, want := range cases {
		if v := m.eval(m.locals[name]); !v.known || v.s != want {
			t.Errorf("local.%s = %+v, want %q", name, v, want)
		}
	}
	if v := m.eval(m.locals["loop"]); v.known {
		t.Error("a reference loop must stay unknown")
	}
}

func TestBrokenInputNeverHangs(t *testing.T) {
	inputs := []string{
		"", "}", ")))", "]", "resource", `resource "a" "b" {`, `x = {`, `x = [`, `x = (`, `x = f(`, `x = "unterminated`,
		"x = <<EOF\nno end", `x = { a = ] }`, `x = [ } ]`, `x = f( ] )`, `x = { for }`, `= = =`, `x = "${`, `x = "${a + "${b}"}"`,
		"resource \"aws_s3_bucket\" \"b\" {\n  acl = \n}\n",
	}
	for _, in := range inputs {
		Stores([]*File{Parse("m/main.tf", []byte(in))})
	}
}

func FuzzParse(f *testing.F) {
	f.Add(`resource "aws_s3_bucket" "b" { bucket = "${var.x}-y" }`)
	f.Add("x = <<EOF\n${a}\nEOF\n")
	f.Add(`x = { a = [1, 2, { b = f(c...) }] }`)
	f.Fuzz(func(t *testing.T, src string) {
		Stores([]*File{Parse("m/main.tf", []byte(src))})
	})
}
