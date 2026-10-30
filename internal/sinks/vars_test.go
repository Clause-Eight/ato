package sinks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// varLang is one language's shape for a sink call reached through a local variable.
type varLang struct {
	path, head, assign, call, tail, field, harmless string
}

// varLangs mirror the generated sink benchmark: per language a function, a local assignment and a log call.
var varLangs = []varLang{
	{"svc/h.go", "func Handle(acct Account, order Order) {", "\tv := %s", "\tlog.Printf(\"done %%v\", %s)", "}", "acct.EmailAddr", "order.%s"},
	{"src/H.java", "  void handle(Account acct, Order order) {", "    var v = %s;", "    log.info(\"done {}\", %s);", "  }", "acct.getEmailAddr()", "order.get%s()"},
	{"web/h.js", "export function handle(acct, order) {", "  const v = %s;", "  console.log('done', %s);", "}", "acct.email_addr", "order.%s"},
	{"web/h.ts", "export function handle(acct: any, order: any) {", "  const v: string = %s;", "  logger.info('done', %s);", "}", "acct.email_addr", "order.%s"},
	{"kt/H.kt", "  fun handle(acct: Account, order: Order) {", "    val v = %s", "    log.info(\"done {}\", %s)", "  }", "acct.email_addr", "order.%s"},
	{"php/H.php", "  public function handle($acct, $order) {", "    $v = %s;", "    $this->logger->info('done', ['v' => %s]);", "  }", "$acct->email_addr", "$order->%s"},
	{"app/h.py", "def handle(acct, order):", "    v = %s", "    log.info(\"done %%s\", %s)", "", "acct.email_addr", "order.%s"},
	{"rb/h.rb", "  def handle(acct, order)", "    v = %s", "    Rails.logger.info(\"done #{%s}\")", "  end", "acct.email_addr", "order.%s"},
}

var harmlessFields = []string{"total", "status", "currency", "created_at", "item_count", "sku", "version", "region"}

func camelCase(f string) string {
	var b strings.Builder
	for _, p := range strings.Split(f, "_") {
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func varVar(l varLang) string {
	if strings.HasSuffix(l.path, ".php") {
		return "$v"
	}
	return "v"
}

func TestThroughVariable(t *testing.T) {
	for _, l := range varLangs {
		code := strings.Join([]string{l.head, fmt.Sprintf(l.assign, l.field), fmt.Sprintf(l.call, varVar(l)), l.tail}, "\n")
		got := scan(l.path, code)
		if len(got) != 1 {
			t.Errorf("%s: got %d findings, want 1: %+v", l.path, len(got), got)
			continue
		}
		g := got[0]
		if g.Sink != domain.SinkLog || g.Category != domain.CategoryContact || g.Location.Line != 3 || g.Masked != varVar(l) {
			t.Errorf("%s: got %+v", l.path, g)
		}
		if len(g.Reasons) != 3 || g.Reasons[1] != "through the variable "+varVar(l)+" assigned on line 2" {
			t.Errorf("%s: reasons %q", l.path, g.Reasons)
		}
	}
}

func TestHarmlessVariables(t *testing.T) {
	for _, l := range varLangs {
		for _, f := range harmlessFields {
			name := f
			if strings.HasSuffix(l.path, ".go") || strings.HasSuffix(l.path, ".java") {
				name = camelCase(f)
			}
			code := strings.Join([]string{l.head, fmt.Sprintf(l.assign, fmt.Sprintf(l.harmless, name)), fmt.Sprintf(l.call, varVar(l)), l.tail}, "\n")
			if got := scan(l.path, code); len(got) != 0 {
				t.Errorf("%s %s: want no findings, got %+v", l.path, f, got)
			}
		}
	}
}

func TestVariableCases(t *testing.T) {
	for _, c := range []struct {
		path, code string
		want       int
	}{
		{"a.go", `e := u.Email; log.Println(e)`, 1},
		{"a.py", "a = acct.email_addr\nb = a\nlog.info(\"%s\", b)", 1},
		{"a.ts", "const a = acct.emailAddr;\nconst b = a;\nlogger.info('x', b);", 1},
		{"a.py", "a = acct.email_addr\nb = a\nb = order.total\nlog.info(\"%s\", b)", 0},
		{"a.py", "a = order.total\nb = a\nlog.info(\"%s\", b)", 0},
		{"a.py", `log.info("%s", acct.email_addr.strip())`, 1},
		{"a.ts", `logger.info('x', acct.emailAddr.trim().toLowerCase());`, 1},
		{"a.ts", `logger.info('x', acct.emailAddr.length);`, 0},
		{"a.py", `log.info("%s", acct.email_addr.split("@")[1])`, 0},
		{"a.go", `log.Println(e); e := u.Email`, 0},
		{"a.go", "e := u.Email\ne = \"x\"\nlog.Println(e)", 0},
		{"a.go", "var e string = u.Email\nlog.Println(e)", 1},
		{"a.py", "v = acct.email_addr\nv = order.total\nlog.info(\"%s\", v)", 0},
		{"a.py", "v = acct.email_addr\nv += \"x\"\nlog.info(\"%s\", v)", 0},
		{"a.py", "v = acct.email_addr\nfor v in rows:\n    log.info(\"%s\", v)", 0},
		{"a.py", "v = acct.email_addr\nlog.info(\"%s\", hash(v))", 0},
		{"a.py", "v = acct.email_addr\nlog.info(\"%s\", len(v))", 0},
		{"a.py", "v = acct.email_addr\nlog.info(f\"done {v}\")", 1},
		{"a.py", "v = mask(acct.email_addr)\nlog.info(\"%s\", v)", 0},
		{"a.py", "self.v = acct.email_addr\nlog.info(\"%s\", v)", 0},
		{"a.py", "v = acct.email_addr\nlog.info(\"%s\", x.v)", 0},
		{"a.py", "v = acct.email_addr\n\ndef other():\n    log.info(\"%s\", v)", 0},
		{"a.py", "v = acct.email_addr\n" + strings.Repeat("x = 1\n", window) + "log.info(\"%s\", v)", 0},
		{"a.java", "String v = acct.getEmailAddr();\nlog.info(\"{}\", v);", 1},
		{"a.java", "var v = acct.getEmailAddr();\nlist.forEach(v -> log.info(\"{}\", v));", 0},
		{"a.java", "var v = acct.getEmailAddr();\n  void other(Order o) {\nlog.info(\"{}\", v);", 0},
		{"a.java", `log.info("{}", customer);`, 1},
		{"a.ts", "const v = acct.email_addr;\nconsole.log(`done ${v}`);", 1},
		{"a.ts", "const v = acct.email_addr;\nconsole.log({ v });", 1},
		{"a.kt", "val v = acct.email_addr\nlog.info(\"done \" + v)", 1},
		{"a.kt", "val v = acct.email_addr\nlog.info(\"done $v\")", 1},
		{"a.kt", `log.warn { "retry for ${acct.phone_no}" }`, 1},
		{"a.kt", `log.warn { "retry for ${order.status}" }`, 0},
		{"a.kt", "val c = acct.phone_no\nlog.warn { \"retry for ${c}\" }", 1},
		{"a.php", "$v = $acct->email_addr;\nerror_log(\"done $v\");", 1},
		{"a.php", "$v = $acct->email_addr;\nforeach ($rows as $v) {\nerror_log(\"done $v\");", 0},
		{"a.rb", "v = acct.email_addr\nRails.logger.info \"done #{v}\"", 1},
		{"a.rb", "v = acct.email_addr\nrows.each { |v| Rails.logger.info(\"#{v}\") }", 0},
		{"a.js", "const v = acct.email_addr;\n// console.log(v)", 0},
	} {
		got := scan(c.path, c.code)
		if len(got) != c.want {
			t.Errorf("%s: %q: got %d findings, want %d: %+v", c.path, c.code, len(got), c.want, got)
		}
	}
}
