package report

import (
	"embed"
	"html/template"
	"io"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

//go:embed html/*
var htmlFiles embed.FS

var htmlTemplates = template.Must(template.New("").Funcs(template.FuncMap{"join": strings.Join}).ParseFS(htmlFiles, "html/*.html"))

func init() { register("html", writeHTML) }

func htmlCSS() template.CSS {
	css, _ := htmlFiles.ReadFile("html/style.css")
	return template.CSS(css)
}

type labelCount struct {
	Label, Country string
	Count          int
}

// writeHTML writes one self-contained page for people who do not use the command line. It loads nothing from
// the network, and its content security policy forbids it to.
func writeHTML(w io.Writer, r Report, _ Options) error {
	files := map[string]bool{}
	counts := map[[2]string]int{}
	high := 0
	for _, f := range r.Findings {
		files[f.Location.Path] = true
		counts[[2]string{f.Label, f.Country}]++
		if f.Confidence == domain.High {
			high++
		}
	}
	byLabel := make([]labelCount, 0, len(counts))
	for k, n := range counts {
		byLabel = append(byLabel, labelCount{Label: k[0], Country: k[1], Count: n})
	}
	sort.Slice(byLabel, func(i, j int) bool {
		if byLabel[i].Count != byLabel[j].Count {
			return byLabel[i].Count > byLabel[j].Count
		}
		return byLabel[i].Label < byLabel[j].Label
	})
	return htmlTemplates.ExecuteTemplate(w, "findings.html", map[string]any{
		"CSS": htmlCSS(), "Tool": r.Tool, "Version": r.Version, "Root": r.Root, "Findings": r.Findings,
		"Files": len(files), "Scanned": r.Summary.FilesScanned, "High": high, "ByLabel": byLabel, "Unreadable": r.Summary.Unreadable,
	})
}

type containerRow struct {
	Name       string
	Categories []string
	Count      int
}

func writeInventoryHTML(w io.Writer, inv Inventory) error {
	rows := map[string]*containerRow{}
	special := 0
	for _, e := range inv.Entries {
		name := e.Field.Container
		if name == "" {
			name = e.Field.Location.Path
		}
		row := rows[name]
		if row == nil {
			row = &containerRow{Name: name}
			rows[name] = row
		}
		row.Count++
		cat := string(e.Classification.Category)
		if e.Classification.Special != "" {
			cat = string(e.Classification.Special) + " (Art. 9)"
			special++
		}
		if !containsString(row.Categories, cat) {
			row.Categories = append(row.Categories, cat)
		}
	}
	containers := make([]containerRow, 0, len(rows))
	for _, r := range rows {
		sort.Strings(r.Categories)
		containers = append(containers, *r)
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	return htmlTemplates.ExecuteTemplate(w, "inventory.html", map[string]any{
		"CSS": htmlCSS(), "Tool": inv.Tool, "Version": inv.Version, "Entries": inv.Entries,
		"Containers": containers, "Special": special, "Files": inv.Files,
	})
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
