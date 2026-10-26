// Package lifecycle finds how a code base removes personal data: deletes and anonymising updates that can
// erase a person's data (GDPR Art. 17), and time-based deletes, TTLs and partition drops that limit how long
// data is kept (Art. 5(1)(e)). It reads code line by line and never runs it, so it shows that a path exists,
// not that it is called.
package lifecycle

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Kinds of evidence.
const (
	Erasure   = "erasure"
	Retention = "retention"
	// Cascade says that deleting a row of Parent deletes the rows of Target that refer to it.
	Cascade = "cascade"
)

// Evidence is one place in the code that removes data from a store.
type Evidence struct {
	Kind     string
	Target   string
	Parent   string
	How      string
	Location domain.Location
}

// Candidate reports whether a file may hold code that deletes data.
func Candidate(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".sql", ".py", ".rb", ".go", ".java", ".kt", ".js", ".mjs", ".cjs", ".ts", ".php", ".cs", ".prisma":
		return true
	}
	return false
}

type rule struct {
	kind string
	how  string
	re   *regexp.Regexp
	// group is the submatch holding the store's name; parent, when set, the submatch holding a cascade's parent.
	group, parent int
	// onlyIf, when set, must also match the line.
	onlyIf *regexp.Regexp
}

// timeBound recognises a condition on a time column, the mark of a delete that keeps data for a period.
var timeBound = regexp.MustCompile(`(?i)(?:_at|_on|date|time|timestamp|created|updated|expir\w*|last_\w+)\W{0,3}\s*(?:<=?|__lte?\b|\blt[e]?:|LessThan|Before)|(?:<=?)\s*(?:now\(\)|current_(?:date|timestamp)|sysdate|getdate\(\)|date_sub|dateadd)|\{\s*lte?\s*:|\b(?:interval|\.ago\b|days?\.ago|timedelta\(|time\.Now\(\)\.Add|Instant\.now\(\)\.minus|LocalDate(?:Time)?\.now\(\)\.minus|subDays|sub\(\s*Duration)`)

var anonymising = regexp.MustCompile(`(?i)=\s*NULL\b|anonymi[sz]|pseudonymi[sz]|redact|'\*+'|md5\(|sha\d*\(|gen_random_uuid\(\)|'deleted'|"deleted"`)

var rules = []rule{
	{kind: Erasure, how: "SQL DELETE", re: regexp.MustCompile("(?i)\\bDELETE\\s+FROM\\s+(?:\\w+\\.)?[\"`\\[]?(\\w+)"), group: 1},
	{kind: Erasure, how: "anonymising SQL UPDATE", re: regexp.MustCompile("(?i)\\bUPDATE\\s+(?:\\w+\\.)?[\"`\\[]?(\\w+)[\"`\\]]?\\s+SET\\b"), group: 1, onlyIf: anonymising},
	{kind: Erasure, how: "Django delete()", re: regexp.MustCompile(`\b([A-Z]\w*)\.objects\b.*\.delete\(\)`), group: 1},
	{kind: Erasure, how: "SQLAlchemy delete", re: regexp.MustCompile(`\b(?:delete|query)\(\s*([A-Z]\w*)\s*\)(?:.*\.delete\(|)`), group: 1, onlyIf: regexp.MustCompile(`\bdelete\(\s*[A-Z]|\.delete\(`)},
	{kind: Erasure, how: "ActiveRecord destroy or delete", re: regexp.MustCompile(`\b([A-Z]\w*)\.(?:where\(.*\)\.|find(?:_by)?\(.*\)\.|find_by_\w+\(.*\)\.)?(?:destroy_all|delete_all|destroy|delete|destroy_by|delete_by)\b`), group: 1},
	{kind: Erasure, how: "GORM Delete", re: regexp.MustCompile(`\.Delete\(\s*&?(?:\[\])?(?:\w+\.)?([A-Z]\w*)\s*\{`), group: 1},
	{kind: Erasure, how: "Spring Data delete", re: regexp.MustCompile(`\b(\w+?)Repository\s*\.\s*delete\w*\(`), group: 1},
	{kind: Erasure, how: "Prisma delete", re: regexp.MustCompile(`\bprisma\.(\w+)\.delete(?:Many)?\(`), group: 1},
	{kind: Erasure, how: "Laravel delete", re: regexp.MustCompile(`\b([A-Z]\w*)::(?:where\(.*\)->|find\(.*\)->|destroy\()(?:delete\(|forceDelete\(|)`), group: 1, onlyIf: regexp.MustCompile(`::destroy\(|->(?:force)?[dD]elete\(`)},
	{kind: Erasure, how: "anonymising routine", re: regexp.MustCompile(`(?i)\b(?:def|func|function|fun|void|public|private|async)\s+(?:\w+\s+)*?(?:anonymi[sz]e|pseudonymi[sz]e|erase|forget|redact)_?([A-Za-z]\w*)\s*\(`), group: 1},
	{kind: Retention, how: "TTL index", re: regexp.MustCompile(`\b(?:db\.)?(\w+)\.create_?[iI]ndex\(.*expire_?[aA]fter_?[sS]econds`), group: 1},
	{kind: Retention, how: "partition drop", re: regexp.MustCompile("(?i)\\bALTER\\s+TABLE\\s+(?:\\w+\\.)?[\"`]?(\\w+)[\"`]?\\s+(?:DROP|DETACH|TRUNCATE)\\s+PARTITION"), group: 1},
	{kind: Retention, how: "pg_partman retention", re: regexp.MustCompile(`(?i)part_config\b.*\bretention\b.*parent_table\s*=\s*'(?:\w+\.)?(\w+)'`), group: 1},
	{kind: Retention, how: "scheduled clean-up", re: regexp.MustCompile(`(?i)\b(?:def|func|function|fun|void|public|private|async)\s+(?:\w+\s+)*?(?:purge|prune|cleanup|clean_up|expire|delete)_?(?:old|expired|stale)_?([A-Za-z]\w*)\s*\(`), group: 1},
	{kind: Cascade, how: "ON DELETE CASCADE", re: regexp.MustCompile("(?i)\\bREFERENCES\\s+(?:\\w+\\.)?[\"`]?(\\w+)[\"`]?\\s*\\([^)]*\\)\\s*ON\\s+DELETE\\s+CASCADE"), parent: 1},
	{kind: Cascade, how: "Django CASCADE", re: regexp.MustCompile(`(?:ForeignKey|OneToOneField)\(\s*['"]?(?:\w+\.)?([A-Z]\w*)['"]?\s*,.*on_delete\s*=\s*models\.CASCADE`), parent: 1},
}

var (
	createTable = regexp.MustCompile("(?i)\\bCREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?(?:\\w+\\.)?[\"`\\[]?(\\w+)")
	alterTable  = regexp.MustCompile("(?i)\\bALTER\\s+TABLE\\s+(?:ONLY\\s+)?(?:\\w+\\.)?[\"`\\[]?(\\w+)")
	pyClass     = regexp.MustCompile(`^class\s+(\w+)\s*\(`)
	railsMany   = regexp.MustCompile(`^\s*has_(?:many|one)\s+:(\w+).*dependent:\s*:(?:destroy|delete_all|delete)`)
	railsClass  = regexp.MustCompile(`^\s*class\s+(\w+)\s*<`)
)

// Find returns the evidence in one file. Cascades remember the table or model they are declared in.
func Find(p string, data []byte) []Evidence {
	var out []Evidence
	container := ""
	for i, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if m := createTable.FindStringSubmatch(l); m != nil {
			container = m[1]
		} else if m := alterTable.FindStringSubmatch(l); m != nil {
			container = m[1]
		} else if m := pyClass.FindStringSubmatch(l); m != nil {
			container = m[1]
		} else if m := railsClass.FindStringSubmatch(l); m != nil {
			container = m[1]
		}
		loc := func(col int) domain.Location { return domain.Location{Path: p, Line: i + 1, Column: col + 1} }
		if m := railsMany.FindStringSubmatchIndex(l); m != nil && container != "" {
			out = append(out, Evidence{Kind: Cascade, Target: l[m[2]:m[3]], Parent: container, How: "Rails dependent: :destroy", Location: loc(m[2])})
		}
		for _, r := range rules {
			if r.onlyIf != nil && !r.onlyIf.MatchString(l) {
				continue
			}
			m := r.re.FindStringSubmatchIndex(l)
			if m == nil {
				continue
			}
			if r.kind == Cascade {
				if container != "" {
					out = append(out, Evidence{Kind: Cascade, Target: container, Parent: l[m[2*r.parent]:m[2*r.parent+1]], How: r.how, Location: loc(m[2*r.parent])})
				}
				continue
			}
			target := l[m[2*r.group]:m[2*r.group+1]]
			kind := r.kind
			if kind == Erasure && timeBound.MatchString(l) {
				kind = Retention
			}
			out = append(out, Evidence{Kind: kind, Target: target, How: r.how, Location: loc(m[2*r.group])})
		}
	}
	return dedupe(out)
}

func dedupe(es []Evidence) []Evidence {
	seen := map[string]bool{}
	var out []Evidence
	for _, e := range es {
		k := e.Kind + "\x00" + Normalize(e.Target) + "\x00" + Normalize(e.Parent) + "\x00" + e.Location.Path + "\x00" + strconv.Itoa(e.Location.Line)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// Normalize makes a table, model or collection name comparable: lower case, no separators, singular.
// "user_profiles", "UserProfile" and "userProfile" all become "userprofile".
func Normalize(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 4:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "sses"), strings.HasSuffix(s, "xes"), strings.HasSuffix(s, "ches"), strings.HasSuffix(s, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && len(s) > 3:
		return s[:len(s)-1]
	}
	return s
}

// Status is what the code shows about removing data from one store.
type Status struct {
	Store     string
	Erasure   []Evidence
	Retention []Evidence
}

// Assess matches evidence to stores by normalised name. A store whose rows are deleted with a parent that can
// be erased counts as erasable through the cascade.
func Assess(stores []string, evidence []Evidence) map[string]Status {
	byName := map[string][]Evidence{}
	parents := map[string][]Evidence{}
	for _, e := range evidence {
		if e.Kind == Cascade {
			parents[Normalize(e.Target)] = append(parents[Normalize(e.Target)], e)
			continue
		}
		byName[Normalize(e.Target)] = append(byName[Normalize(e.Target)], e)
	}
	var erasable func(name string, depth int) []Evidence
	erasable = func(name string, depth int) []Evidence {
		var out []Evidence
		for _, e := range byName[name] {
			if e.Kind == Erasure {
				out = append(out, e)
			}
		}
		if len(out) > 0 || depth > 4 {
			return out
		}
		for _, c := range parents[name] {
			if up := erasable(Normalize(c.Parent), depth+1); len(up) > 0 {
				out = append(out, c)
			}
		}
		return out
	}
	out := map[string]Status{}
	for _, s := range stores {
		n := Normalize(s)
		st := Status{Store: s, Erasure: erasable(n, 0)}
		for _, e := range byName[n] {
			if e.Kind == Retention {
				st.Retention = append(st.Retention, e)
			}
		}
		sort.SliceStable(st.Erasure, func(i, j int) bool { return st.Erasure[i].Location.Path < st.Erasure[j].Location.Path })
		out[s] = st
	}
	return out
}
