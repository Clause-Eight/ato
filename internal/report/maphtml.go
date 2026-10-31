package report

import (
	"fmt"
	"html/template"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/datamap"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// The HTML data map is written for a data protection officer: what changed since the last saved map, what needs a
// decision, then the map itself, in words rather than code. It shows fields, stores and file locations, never
// values: a data map holds no values.

type place struct {
	Path string
	Line int
}

// Href links to the file relative to the scanned folder. It always starts with "./" so it can never be read as
// another site or a scheme; an absolute path gets no link.
func (p place) Href() string {
	path := p.Path
	if i := strings.Index(path, "!/"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimPrefix(path, "./")
	if strings.HasPrefix(path, "/") || path == "" {
		return ""
	}
	return "./" + (&url.URL{Path: path}).EscapedPath()
}

func (p place) Text() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s, line %d", p.Path, p.Line)
	}
	return p.Path
}

func placeOf(l domain.Location) place { return place{Path: l.Path, Line: l.Line} }

type changeItem struct {
	Text  string
	Where []place
}

type changeGroup struct {
	Title string
	Items []changeItem
}

type decision struct {
	Title, Why, Decide string
	Where              []place
}

type mapPage struct {
	CSS                                  template.CSS
	Tool, Version                        string
	Compared                             bool
	PreviousName                         string
	Changes                              []changeGroup
	Decisions                            []decision
	Components                           []componentView
	Stores, Recipients, Sinks, Infra     int
	Kinds                                string
	Special, SpecialLaw                  string
	ChangeCount, AddedCount, RemoveCount int
}

type componentView struct {
	Name       string
	Stores     []storeView
	Recipients []recipientView
	Infra      []infraView
	Sinks      []sinkView
}

type storeView struct {
	Name, Kinds, Special, SpecialLaw string
	Erasure, Retention               string
	ErasureAt                        []place
	RetentionAt                      []place
	Fields                           []fieldView
}

type fieldView struct {
	Name, Kind string
	Special    bool
	Where      place
}

type recipientView struct {
	Name, Purpose, EEA string
	Where              []place
	More               int
}

type infraView struct {
	Address, Service, Regions                 string
	InEEA, Encrypted, Retention, Public, Data string
	Where                                     place
}

type sinkView struct {
	Label, Kind string
	Where       place
}

var categoryText = map[string]string{
	string(domain.CategoryName):        "names",
	string(domain.CategoryContact):     "contact details such as email or phone",
	string(domain.CategoryAddress):     "postal addresses",
	string(domain.CategoryIdentifier):  "government ID numbers",
	string(domain.CategoryFinancial):   "financial data such as bank or card numbers",
	string(domain.CategoryBirth):       "dates of birth",
	string(domain.CategoryLocation):    "location data",
	string(domain.CategoryOnline):      "online identifiers such as IP addresses or device IDs",
	string(domain.CategoryDemographic): "demographic data such as age or gender",
	string(domain.CategorySpecial):     "special-category data",
}

var specialText = map[string]string{
	string(domain.SpecialHealth):       "health data",
	string(domain.SpecialGenetic):      "genetic data",
	string(domain.SpecialBiometric):    "biometric data",
	string(domain.SpecialRacialEthnic): "racial or ethnic origin",
	string(domain.SpecialPolitical):    "political opinions",
	string(domain.SpecialReligious):    "religious or philosophical beliefs",
	string(domain.SpecialTradeUnion):   "trade union membership",
	string(domain.SpecialSexLife):      "sex life or sexual orientation",
	string(domain.SpecialCriminal):     "criminal convictions and offences",
}

// articles names the GDPR articles that restrict the special categories: Art. 9, and Art. 10 for criminal data.
func articles(specials []string) string {
	nine, ten := false, false
	for _, sp := range specials {
		if domain.SpecialCategory(sp) == domain.SpecialCriminal {
			ten = true
		} else {
			nine = true
		}
	}
	switch {
	case ten && nine:
		return "GDPR Art. 9 and 10"
	case ten:
		return "GDPR Art. 10"
	}
	return "GDPR Art. 9"
}

func categoryWords(c string) string {
	if w, ok := categoryText[c]; ok {
		return w
	}
	return strings.ReplaceAll(c, "-", " ")
}

func specialWords(s string) string {
	if w, ok := specialText[s]; ok {
		return w
	}
	return strings.ReplaceAll(s, "-", " ")
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func wordList(xs []string, words func(string) string) string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, words(x))
	}
	return joinAnd(out)
}

func joinAnd(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

func settingWords(s string) string {
	switch s {
	case "yes":
		return "yes"
	case "no":
		return "no"
	case "unknown":
		return "not set in the code"
	}
	return "does not apply"
}

func writeMapHTML(w io.Writer, m DataMap) error {
	page := mapPage{CSS: htmlCSS() + mapCSS(), Tool: Product, Version: m.Version}
	var cats, specials []string
	for _, c := range m.Map.Categories() {
		if sp, ok := strings.CutPrefix(c, string(domain.CategorySpecial)+"/"); ok {
			specials = append(specials, sp)
		} else {
			cats = append(cats, c)
		}
	}
	page.Kinds, page.Special, page.SpecialLaw = wordList(cats, categoryWords), wordList(specials, specialWords), articles(specials)
	for _, c := range m.Map.Components {
		page.Stores += len(c.Stores)
		page.Recipients += len(c.Recipients)
		page.Sinks += len(c.Sinks)
		page.Infra += len(c.Infra)
		page.Components = append(page.Components, componentPage(c))
	}
	if m.Previous != nil {
		cmp := datamap.Compare(*m.Previous, m.Map)
		page.Compared, page.PreviousName = true, m.PreviousName
		page.Changes = changeGroups(*m.Previous, m.Map, cmp)
		page.AddedCount, page.RemoveCount = len(cmp.Added)+len(cmp.Weakened), len(cmp.Removed)
		page.ChangeCount = page.AddedCount + page.RemoveCount
		page.Decisions = changeDecisions(m.Map, cmp)
	} else {
		page.Decisions = mapDecisions(m.Map)
	}
	return htmlTemplates.ExecuteTemplate(w, "map.html", page)
}

func mapCSS() template.CSS {
	css, _ := htmlFiles.ReadFile("html/map.css")
	return template.CSS(css)
}

func removalWords(assessed bool, rs []domain.Removal, none string) (string, []place) {
	switch {
	case !assessed:
		return "Not checked.", nil
	case len(rs) == 0:
		return none, nil
	}
	text := rs[0].How
	if len(rs) > 1 {
		text += fmt.Sprintf(" (and %d more)", len(rs)-1)
	}
	return text + ".", []place{placeOf(rs[0].Location)}
}

func componentPage(c datamap.Component) componentView {
	out := componentView{Name: c.Name}
	for _, s := range c.Stores {
		sv := storeView{Name: s.Name, Kinds: wordList(s.Categories, categoryWords), Special: wordList(s.Special, specialWords), SpecialLaw: articles(s.Special)}
		sv.Erasure, sv.ErasureAt = removalWords(s.Assessed, s.Erasure, "No code found that erases a person's data.")
		sv.Retention, sv.RetentionAt = removalWords(s.Assessed, s.Retention, "No code found that removes old data.")
		for _, f := range s.Fields {
			fv := fieldView{Name: f.Name, Kind: categoryWords(f.Category), Where: placeOf(f.Location)}
			if f.Special != "" {
				fv.Kind, fv.Special = specialWords(f.Special), true
			}
			sv.Fields = append(sv.Fields, fv)
		}
		out.Stores = append(out.Stores, sv)
	}
	for _, r := range c.Recipients {
		rv := recipientView{Name: r.Name, Purpose: r.Purpose, EEA: "yes"}
		if !r.EEA {
			rv.EEA = "no: check the transfer safeguard"
		}
		for i, e := range r.Evidence {
			if i == 3 {
				rv.More = len(r.Evidence) - 3
				break
			}
			rv.Where = append(rv.Where, placeOf(e))
		}
		out.Recipients = append(out.Recipients, rv)
	}
	for _, s := range c.Infra {
		regions := strings.Join(s.Regions, ", ")
		if regions == "" {
			regions = "not set in the code"
		}
		out.Infra = append(out.Infra, infraView{
			Address: s.Address, Service: strings.ReplaceAll(s.Service, "-", " "), Regions: regions,
			InEEA: settingWords(s.InEEA), Encrypted: settingWords(s.Encrypted), Retention: settingWords(s.Retention),
			Public: settingWords(s.Public), Data: yesNo(s.PersonalData), Where: placeOf(s.Location),
		})
	}
	for _, s := range c.Sinks {
		out.Sinks = append(out.Sinks, sinkView{Label: s.Label, Kind: categoryWords(s.Category), Where: placeOf(s.Location)})
	}
	return out
}

func component(m datamap.Map, name string) datamap.Component {
	for _, c := range m.Components {
		if c.Name == name {
			return c
		}
	}
	return datamap.Component{Name: name}
}

// evidence finds where a change shows in a map: the fields of a category, the places a recipient is named, a
// store's first field, a Terraform resource or the logging calls. It returns at most three places.
func evidence(m datamap.Map, ch datamap.Change) (out []place) {
	c := component(m, ch.Component)
	add := func(l domain.Location) { out = append(out, placeOf(l)) }
	defer func() {
		sort.Slice(out, func(i, j int) bool {
			if out[i].Path != out[j].Path {
				return out[i].Path < out[j].Path
			}
			return out[i].Line < out[j].Line
		})
		if len(out) > 3 {
			out = out[:3]
		}
	}()
	switch ch.Kind {
	case datamap.NewCategory, datamap.NewSpecial:
		for _, s := range c.Stores {
			for _, f := range s.Fields {
				if ch.Kind == datamap.NewCategory && f.Special == "" && f.Category == ch.Name || ch.Kind == datamap.NewSpecial && f.Special == ch.Name {
					add(f.Location)
				}
			}
		}
	case datamap.NewRecipient:
		for _, r := range c.Recipients {
			if r.Name == ch.Name {
				for _, e := range r.Evidence {
					add(e)
				}
			}
		}
	case datamap.NewStore:
		for _, s := range c.Stores {
			if s.Name == ch.Name && len(s.Fields) > 0 {
				add(s.Fields[0].Location)
			}
		}
	case datamap.NewInfra:
		for _, s := range c.Infra {
			if s.Address == ch.Name {
				add(s.Location)
			}
		}
	case datamap.NewSink:
		for _, s := range c.Sinks {
			if s.Category == ch.Name {
				add(s.Location)
			}
		}
	}
	return out
}

func recipientOf(m datamap.Map, comp, name string) datamap.Recipient {
	for _, r := range component(m, comp).Recipients {
		if r.Name == name {
			return r
		}
	}
	return datamap.Recipient{Name: name}
}

func recipientText(r datamap.Recipient, comp string) string {
	text := r.Name
	if r.Purpose != "" {
		text += " (" + r.Purpose + ")"
	}
	text += ", named in " + comp
	if !r.EEA {
		text += ". Not established in the EEA"
	}
	return text + "."
}

var lostText = map[string]string{
	datamap.LostEncryption: "is no longer encrypted",
	datamap.LostRetention:  "no longer has a rule that deletes old data",
	datamap.LostEEA:        "is no longer limited to EU regions",
	datamap.LostErasure:    "no longer has code that erases a person's data",
	datamap.LostExpiry:     "no longer has code that removes old data",
}

func changeGroups(previous, current datamap.Map, cmp datamap.Comparison) []changeGroup {
	groups := []changeGroup{
		{Title: "New kinds of personal data"},
		{Title: "New special-category data"},
		{Title: "New outside companies that may receive data"},
		{Title: "New stores, and stores that lost a protection"},
		{Title: "New places personal data is written to logs"},
		{Title: "Removed since the last saved map"},
	}
	for _, ch := range cmp.Added {
		where := evidence(current, ch)
		switch ch.Kind {
		case datamap.NewCategory:
			groups[0].Items = append(groups[0].Items, changeItem{upperFirst(categoryWords(ch.Name)) + " in " + ch.Component + ".", where})
		case datamap.NewSpecial:
			groups[1].Items = append(groups[1].Items, changeItem{upperFirst(specialWords(ch.Name)) + " in " + ch.Component + ".", where})
		case datamap.NewRecipient:
			groups[2].Items = append(groups[2].Items, changeItem{recipientText(recipientOf(current, ch.Component, ch.Name), ch.Component), where})
		case datamap.NewStore:
			groups[3].Items = append(groups[3].Items, changeItem{"New store " + ch.Name + " in " + ch.Component + ".", where})
		case datamap.NewInfra:
			groups[3].Items = append(groups[3].Items, changeItem{"New cloud store " + ch.Name + " in " + ch.Component + ", linked to personal data.", where})
		case datamap.NewSink:
			groups[4].Items = append(groups[4].Items, changeItem{"The code in " + ch.Component + " writes " + categoryWords(ch.Name) + " to logs or telemetry.", where})
		}
	}
	for _, wk := range cmp.Weakened {
		groups[3].Items = append(groups[3].Items, changeItem{"The store " + wk.Store + " in " + wk.Component + " " + lostText[wk.Lost] + ".", weakEvidence(current, wk)})
	}
	for _, ch := range cmp.Removed {
		var text string
		switch ch.Kind {
		case datamap.NewCategory:
			text = "Kind of data no longer found: " + categoryWords(ch.Name) + " in " + ch.Component + "."
		case datamap.NewSpecial:
			text = "Special-category data no longer found: " + specialWords(ch.Name) + " in " + ch.Component + "."
		case datamap.NewRecipient:
			text = ch.Name + " is no longer named in " + ch.Component + "."
		case datamap.NewStore:
			text = "Store " + ch.Name + " in " + ch.Component + " is gone."
		case datamap.NewInfra:
			text = "Cloud store " + ch.Name + " in " + ch.Component + " is gone or no longer linked to personal data."
		case datamap.NewSink:
			text = "No longer written to logs: " + categoryWords(ch.Name) + " in " + ch.Component + "."
		}
		groups[5].Items = append(groups[5].Items, changeItem{text, evidence(previous, ch)})
	}
	var added []string
	for _, ch := range cmp.Added {
		if ch.Kind == datamap.NewSpecial {
			added = append(added, ch.Name)
		}
	}
	groups[1].Title += " (" + articles(added) + ")"
	out := groups[:0]
	for _, g := range groups {
		if len(g.Items) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func weakEvidence(m datamap.Map, wk datamap.Weakening) []place {
	c := component(m, wk.Component)
	for _, s := range c.Infra {
		if s.Address == wk.Store {
			return []place{placeOf(s.Location)}
		}
	}
	for _, s := range c.Stores {
		if s.Name == wk.Store && len(s.Fields) > 0 {
			return []place{placeOf(s.Fields[0].Location)}
		}
	}
	return nil
}

func categoryDecision(ch datamap.Change, where []place) decision {
	words := categoryWords(ch.Name)
	return decision{
		Title:  "New kind of data: " + words + " in " + ch.Component,
		Why:    "The code now holds " + words + " in " + ch.Component + ", which the last saved map did not show.",
		Decide: "Decide the purpose and the legal basis (GDPR Art. 6), and add them to the record of processing.",
		Where:  where,
	}
}

func specialDecision(comp, sp string, where []place) decision {
	words := specialWords(sp)
	if domain.SpecialCategory(sp) == domain.SpecialCriminal {
		return decision{
			Title:  "Criminal data: " + words + " in " + comp,
			Why:    upperFirst(words) + " may only be processed under the control of official authority or where the law authorises it (GDPR Art. 10).",
			Decide: "Decide the purpose, which law authorises the processing and whether a data protection impact assessment is needed.",
			Where:  where,
		}
	}
	return decision{
		Title:  "Special-category data: " + words + " in " + comp,
		Why:    upperFirst(words) + " is special-category data (GDPR Art. 9), which may only be processed under one of the exceptions in Art. 9(2).",
		Decide: "Decide the purpose, which Art. 9(2) exception applies and whether a data protection impact assessment is needed.",
		Where:  where,
	}
}

func recipientDecision(comp string, r datamap.Recipient, where []place) decision {
	d := decision{
		Title:  "Outside company: " + r.Name,
		Why:    "The code in " + comp + " names " + r.Name + ", so personal data may be sent to it.",
		Decide: "Check there is a processor agreement (GDPR Art. 28) and that it is listed as a recipient.",
		Where:  where,
	}
	if r.Purpose != "" {
		d.Title += " (" + r.Purpose + ")"
	}
	if !r.EEA {
		d.Why += " It is not established in the EEA."
		d.Decide += " Also check the safeguard for transfers outside the EEA (Chapter V)."
	}
	return d
}

func sinkDecision(comp, cat string, where []place) decision {
	words := categoryWords(cat)
	return decision{
		Title:  "Personal data in logs: " + words + " in " + comp,
		Why:    "The code writes " + words + " to logs or telemetry, which are often kept longer and seen by more people than the data itself.",
		Decide: "Decide whether logs may hold this data; if not, ask for it to be removed or masked.",
		Where:  where,
	}
}

// storeDecision is set when the code was checked and has no erasure path, no retention path or neither.
func storeDecision(comp string, s datamap.Store) (decision, bool) {
	if !s.Assessed || len(s.Erasure) > 0 && len(s.Retention) > 0 {
		return decision{}, false
	}
	d := decision{Title: "Store without erasure or retention: " + s.Name + " in " + comp}
	switch {
	case len(s.Erasure) == 0 && len(s.Retention) == 0:
		d.Why = "No code was found that erases a person's data from " + s.Name + " or removes old data from it."
		d.Decide = "Decide how a person's data is erased on request (GDPR Art. 17) and how long it is kept (Art. 5(1)(e))."
	case len(s.Erasure) == 0:
		d.Why = "No code was found that erases a person's data from " + s.Name + "."
		d.Decide = "Decide how a person's data here is erased on request (GDPR Art. 17)."
	default:
		d.Why = "No code was found that removes old data from " + s.Name + "."
		d.Decide = "Decide how long this data is kept and ask for a retention limit (GDPR Art. 5(1)(e))."
	}
	if len(s.Fields) > 0 {
		d.Where = []place{placeOf(s.Fields[0].Location)}
	}
	return d, true
}

// infraDecision is set when a Terraform store linked to personal data is unencrypted, has no retention rule, is
// outside the EEA or is public.
func infraDecision(comp string, s datamap.Infra) (decision, bool) {
	if !s.PersonalData {
		return decision{}, false
	}
	var issues, asks []string
	if s.Encrypted == "no" {
		issues, asks = append(issues, "is not encrypted"), append(asks, "require encryption at rest")
	}
	if s.Retention == "no" {
		issues, asks = append(issues, "has no rule that deletes old data"), append(asks, "set a retention limit")
	}
	if s.InEEA == "no" {
		issues = append(issues, "is outside the EEA ("+strings.Join(s.Regions, ", ")+")")
		asks = append(asks, "move it to an EU region or record the transfer safeguard")
	}
	if s.Public == "yes" {
		issues, asks = append(issues, "is public"), append(asks, "confirm it must be public")
	}
	if len(issues) == 0 {
		return decision{}, false
	}
	return decision{
		Title:  "Cloud store: " + s.Address + " in " + comp,
		Why:    "It holds personal data and " + joinAnd(issues) + ".",
		Decide: "Decide whether to " + joinAnd(asks) + ".",
		Where:  []place{placeOf(s.Location)},
	}, true
}

var weakDecide = map[string]string{
	datamap.LostEncryption: "Decide whether encryption at rest is required, and if so ask for it back.",
	datamap.LostRetention:  "Decide the retention limit and ask for the rule back.",
	datamap.LostEEA:        "Decide whether this data may leave the EEA; if so, record the transfer safeguard (GDPR Chapter V).",
	datamap.LostErasure:    "Decide how erasure requests (GDPR Art. 17) are handled for this store.",
	datamap.LostExpiry:     "Decide how long this data is kept and how it is removed (GDPR Art. 5(1)(e)).",
}

// changeDecisions lists what needs a decision because of a change since the previous map.
func changeDecisions(m datamap.Map, cmp datamap.Comparison) []decision {
	var special, other []decision
	for _, ch := range cmp.Added {
		where := evidence(m, ch)
		c := component(m, ch.Component)
		switch ch.Kind {
		case datamap.NewSpecial:
			special = append(special, specialDecision(ch.Component, ch.Name, where))
		case datamap.NewCategory:
			other = append(other, categoryDecision(ch, where))
		case datamap.NewRecipient:
			other = append(other, recipientDecision(ch.Component, recipientOf(m, ch.Component, ch.Name), where))
		case datamap.NewSink:
			other = append(other, sinkDecision(ch.Component, ch.Name, where))
		case datamap.NewStore:
			for _, s := range c.Stores {
				if d, ok := storeDecision(ch.Component, s); ok && s.Name == ch.Name {
					other = append(other, d)
				}
			}
		case datamap.NewInfra:
			for _, s := range c.Infra {
				if d, ok := infraDecision(ch.Component, s); ok && s.Address == ch.Name {
					other = append(other, d)
				}
			}
		}
	}
	// One decision per store, however many protections it lost.
	for i := 0; i < len(cmp.Weakened); {
		wk := cmp.Weakened[i]
		var lost, decide []string
		for ; i < len(cmp.Weakened) && cmp.Weakened[i].Component == wk.Component && cmp.Weakened[i].Store == wk.Store; i++ {
			lost, decide = append(lost, lostText[cmp.Weakened[i].Lost]), append(decide, weakDecide[cmp.Weakened[i].Lost])
		}
		other = append(other, decision{
			Title:  "Lost protection: " + wk.Store + " in " + wk.Component,
			Why:    "The store " + wk.Store + " " + joinAnd(lost) + ".",
			Decide: strings.Join(decide, " "),
			Where:  weakEvidence(m, wk),
		})
	}
	return append(special, other...)
}

// mapDecisions lists what needs a decision in a map with nothing to compare with: special-category data, outside
// companies, stores without erasure or retention, cloud stores without protections, and logging of personal data.
func mapDecisions(m datamap.Map) []decision {
	var special, recipients, stores, sinks []decision
	for _, c := range m.Components {
		for _, s := range c.Stores {
			for _, sp := range s.Special {
				special = append(special, specialDecision(c.Name, sp, evidence(m, datamap.Change{Component: c.Name, Kind: datamap.NewSpecial, Name: sp})))
			}
			if d, ok := storeDecision(c.Name, s); ok {
				stores = append(stores, d)
			}
		}
		for _, r := range c.Recipients {
			recipients = append(recipients, recipientDecision(c.Name, r, evidence(m, datamap.Change{Component: c.Name, Kind: datamap.NewRecipient, Name: r.Name})))
		}
		for _, s := range c.Infra {
			if d, ok := infraDecision(c.Name, s); ok {
				stores = append(stores, d)
			}
		}
		seen := map[string]bool{}
		for _, s := range c.Sinks {
			if !seen[s.Category] {
				seen[s.Category] = true
				sinks = append(sinks, sinkDecision(c.Name, s.Category, evidence(m, datamap.Change{Component: c.Name, Kind: datamap.NewSink, Name: s.Category})))
			}
		}
	}
	out := append(special, recipients...)
	out = append(out, stores...)
	return append(out, sinks...)
}
