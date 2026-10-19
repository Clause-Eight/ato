// Package cli is the only place that reads arguments, prints and decides exit codes.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/baseline"
	"github.com/Clause-Eight/pii-eu/internal/config"
	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
	"github.com/Clause-Eight/pii-eu/internal/engine"
	"github.com/Clause-Eight/pii-eu/internal/report"
	"github.com/Clause-Eight/pii-eu/internal/walk"
)

// Exit codes. They are part of the public contract.
const (
	ExitClean      = 0
	ExitFindings   = 1
	ExitUsage      = 2
	ExitIncomplete = 3
)

// Version is set at build time.
var Version = "dev"

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// Run executes the command line and returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitUsage
	}
	var code int
	var err error
	switch args[0] {
	case "scan":
		code, err = runScan(args[1:], stdout, stderr)
	case "baseline":
		code, err = runBaseline(args[1:], stdout, stderr)
	case "detectors":
		code, err = runDetectors(args[1:], stdout)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "pii-eu %s\n", Version)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
	default:
		err = usageError{fmt.Sprintf("unknown command %q", args[0])}
	}
	if err == nil {
		return code
	}
	if errors.Is(err, flag.ErrHelp) {
		return ExitClean
	}
	fmt.Fprintf(stderr, "pii-eu: %s\n", err)
	var ue usageError
	if errors.As(err, &ue) {
		fmt.Fprintln(stderr, "Run `pii-eu help` for usage.")
	}
	return ExitUsage
}

const usage = `pii-eu finds EU personal data in code, fixtures and logs.

Usage:
  pii-eu scan [flags] [path]   scan a folder or file (default: current folder); "-" reads stdin
  pii-eu baseline [path]       accept current findings so only new ones are reported
  pii-eu detectors [--markdown]  list what can be detected
  pii-eu version               print the version

Scan flags:
  --config FILE            settings file (default: <path>/.pii-eu.toml)
  --include GLOB           only scan matching paths (repeatable)
  --exclude GLOB           skip matching paths (repeatable)
  --min-confidence LEVEL   report low, medium or high and above (default: medium)
  --countries LIST         only run detectors for these countries, e.g. ro,bg or a preset: cee, dach,
                           benelux, nordics, baltics, eu, eea (default: every EU country)
  --format FORMAT          text, json or sarif (default: text)
  --output FILE            write the report to a file instead of stdout
  --no-git                 walk the folder instead of asking git for the file list
  --staged                 scan the staged version of staged files (for pre-commit hooks)
  --baseline FILE          baseline file (default: <path>/.pii-eu-baseline.json)
  --show-values            print found values in full in text output (never in files)

Exit codes: 0 nothing found, 1 findings, 2 usage or settings error, 3 some files could not be read.
`

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

type scanFlags struct {
	configPath    string
	include       multi
	exclude       multi
	minConfidence string
	countries     multi
	format        string
	output        string
	noGit         bool
	showValues    bool
	staged        bool
	baselinePath  string
}

func parseScanFlags(args []string, stderr io.Writer) (scanFlags, string, error) {
	var f scanFlags
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.configPath, "config", "", "")
	fs.Var(&f.include, "include", "")
	fs.Var(&f.exclude, "exclude", "")
	fs.StringVar(&f.minConfidence, "min-confidence", "", "")
	fs.Var(&f.countries, "countries", "")
	fs.StringVar(&f.format, "format", "text", "")
	fs.StringVar(&f.output, "output", "", "")
	fs.BoolVar(&f.noGit, "no-git", false, "")
	fs.BoolVar(&f.showValues, "show-values", false, "")
	fs.BoolVar(&f.staged, "staged", false, "")
	fs.StringVar(&f.baselinePath, "baseline", "", "")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	if err := fs.Parse(reorder(args)); err != nil {
		return f, "", err
	}
	switch fs.NArg() {
	case 0:
		return f, ".", nil
	case 1:
		return f, fs.Arg(0), nil
	}
	return f, "", usageError{"scan takes one path"}
}

// reorder moves flags in front of the path so `pii-eu scan . --format json` works.
func reorder(args []string) []string {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && takesValue(a) && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		rest = append(rest, a)
	}
	return append(flags, rest...)
}

func takesValue(flagName string) bool {
	switch strings.TrimLeft(flagName, "-") {
	case "no-git", "show-values", "staged":
		return false
	}
	return true
}

func loadConfig(f scanFlags, root string) (config.Config, error) {
	path, explicit := f.configPath, true
	if path == "" {
		explicit = false
		dir := root
		if st, err := os.Stat(root); err == nil && !st.IsDir() {
			dir = filepath.Dir(root)
		}
		path = filepath.Join(dir, config.FileName)
	}
	cfg, err := config.Load(path, explicit)
	if err != nil {
		return cfg, usageError{err.Error()}
	}
	cfg.Include = append(cfg.Include, f.include...)
	cfg.Exclude = append(cfg.Exclude, f.exclude...)
	if f.minConfidence != "" {
		c, err := domain.ParseConfidence(f.minConfidence)
		if err != nil {
			return cfg, usageError{err.Error()}
		}
		cfg.MinConfidence = c
	}
	if len(f.countries) > 0 {
		var items []string
		for _, v := range f.countries {
			items = append(items, strings.Split(v, ",")...)
		}
		countries, err := domain.ParseCountries(items)
		if err != nil {
			return cfg, usageError{err.Error()}
		}
		cfg.Countries = countries
	}
	return cfg, nil
}

func enabledDetectors(cfg config.Config) []detectors.Detector {
	var out []detectors.Detector
	for _, d := range detectors.All() {
		if cfg.Enabled(d.Name, d.Countries(), d.OnByDefault) {
			out = append(out, d)
		}
	}
	return out
}

func runScan(args []string, stdout, stderr io.Writer) (int, error) {
	f, root, err := parseScanFlags(args, stderr)
	if err != nil {
		return 0, err
	}
	res, cfg, err := collect(f, root)
	if err != nil {
		return 0, err
	}
	findings, ignored := withoutIgnored(res.Findings)
	res.Summary.Ignored = ignored
	if root != "-" {
		b, err := baseline.Load(baselinePath(f, root), f.baselinePath != "")
		if err != nil {
			return 0, usageError{err.Error()}
		}
		findings, res.Summary.Baselined = b.Filter(findings)
	}
	findings = atLeast(findings, cfg)
	if err := writeReport(f, report.Report{Tool: "pii-eu", Version: Version, Root: root, Findings: findings, Summary: res.Summary}, stdout); err != nil {
		return 0, err
	}
	switch {
	case len(findings) > 0:
		return ExitFindings, nil
	case len(res.Summary.Unreadable) > 0:
		return ExitIncomplete, nil
	}
	return ExitClean, nil
}

func collect(f scanFlags, root string) (engine.Result, config.Config, error) {
	configRoot := root
	if root == "-" {
		configRoot = "."
	}
	cfg, err := loadConfig(f, configRoot)
	if err != nil {
		return engine.Result{}, cfg, err
	}
	for _, names := range []map[string]bool{cfg.Detectors, keys(cfg.DetectorMinConfidence)} {
		for name := range names {
			if _, ok := detectors.Get(name); !ok {
				return engine.Result{}, cfg, usageError{fmt.Sprintf("unknown detector %q in settings: run `pii-eu detectors`", name)}
			}
		}
	}
	dets := enabledDetectors(cfg)
	if root == "-" {
		fs, err := engine.ScanReader("<stdin>", Stdin, dets)
		return engine.Result{Findings: fs, Summary: domain.Summary{FilesScanned: 1}}, cfg, err
	}
	res, err := engine.Scan(engine.Request{
		Root:      root,
		Walk:      walk.Options{Include: cfg.Include, Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize, NoGit: f.noGit, Staged: f.staged},
		Detectors: dets,
	})
	if errors.Is(err, walk.ErrBadGlob) || errors.Is(err, walk.ErrNotGit) {
		return res, cfg, usageError{err.Error()}
	}
	return res, cfg, err
}

// Stdin is read by `pii-eu scan -`. Tests replace it.
var Stdin io.Reader = os.Stdin

func baselinePath(f scanFlags, root string) string {
	if f.baselinePath != "" {
		return f.baselinePath
	}
	dir := root
	if st, err := os.Stat(root); err == nil && !st.IsDir() {
		dir = filepath.Dir(root)
	}
	return filepath.Join(dir, baseline.FileName)
}

func withoutIgnored(fs []domain.Finding) ([]domain.Finding, int) {
	var out []domain.Finding
	n := 0
	for _, f := range fs {
		if f.Ignored {
			n++
			continue
		}
		out = append(out, f)
	}
	return out, n
}

func runBaseline(args []string, stdout, stderr io.Writer) (int, error) {
	f, root, err := parseScanFlags(args, stderr)
	if err != nil {
		return 0, err
	}
	if root == "-" {
		return 0, usageError{"a baseline needs a folder or file, not stdin"}
	}
	res, _, err := collect(f, root)
	if err != nil {
		return 0, err
	}
	findings, _ := withoutIgnored(res.Findings)
	path := baselinePath(f, root)
	if err := baseline.Save(path, findings); err != nil {
		return 0, fmt.Errorf("cannot write %s: %w", path, err)
	}
	fmt.Fprintf(stdout, "Baseline written to %s with %d %s. Commit it so CI reports only new findings.\n", path, len(findings), plural(len(findings), "finding", "findings"))
	return ExitClean, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func keys[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func atLeast(fs []domain.Finding, cfg config.Config) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Confidence >= cfg.MinConfidenceFor(f.Detector) {
			out = append(out, f)
		}
	}
	return out
}

func writeReport(f scanFlags, r report.Report, stdout io.Writer) error {
	opts := report.Options{ShowValues: f.showValues && f.output == ""}
	if f.output == "" {
		if err := report.Write(f.format, stdout, r, opts); err != nil {
			return usageError{err.Error()}
		}
		return nil
	}
	file, err := os.Create(f.output)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", f.output, err)
	}
	if err := report.Write(f.format, file, r, opts); err != nil {
		file.Close()
		return usageError{err.Error()}
	}
	return file.Close()
}

func runDetectors(args []string, stdout io.Writer) (int, error) {
	markdown := len(args) == 1 && args[0] == "--markdown"
	if len(args) > 0 && !markdown {
		return 0, usageError{"detectors takes only --markdown"}
	}
	if markdown {
		_, err := io.WriteString(stdout, DetectorTable())
		return ExitClean, err
	}
	var b strings.Builder
	for _, d := range detectors.All() {
		state := ""
		if !d.OnByDefault {
			state = " (off by default)"
		}
		fmt.Fprintf(&b, "%-16s %-3s %-7s %s%s\n", d.Name, d.Country, d.Base, d.Label, state)
	}
	_, err := io.WriteString(stdout, b.String())
	return ExitClean, err
}

// coverageColumns are the kinds of national identifier shown in the README's coverage matrix.
var coverageColumns = []struct {
	title   string
	concept detectors.Concept
}{
	{"Personal number", detectors.NationalID},
	{"Tax ID", detectors.TaxID},
	{"Social security or health", detectors.SocialSecurity},
	{"ID card or passport", detectors.IDCard},
}

// CoverageMatrix is the table of national identifiers per country in the README, generated from the registry.
// A detector appears under the first of its concepts that has a column, or under the last column otherwise.
func CoverageMatrix() string {
	cells := map[string]map[int][]string{}
	for _, d := range detectors.All() {
		if d.Country == domain.CrossBorder {
			continue
		}
		col := len(coverageColumns) - 1
		for _, c := range d.Concepts {
			if i := columnOf(c); i >= 0 {
				col = i
				break
			}
		}
		name := "`" + d.Name + "`"
		if !d.OnByDefault {
			name += "*"
		}
		for _, cc := range d.Countries() {
			if cells[cc] == nil {
				cells[cc] = map[int][]string{}
			}
			cells[cc][col] = append(cells[cc][col], name)
		}
	}
	var b strings.Builder
	b.WriteString("| Country |")
	for _, c := range coverageColumns {
		b.WriteString(" " + c.title + " |")
	}
	b.WriteString("\n|---|" + strings.Repeat("---|", len(coverageColumns)) + "\n")
	for _, cc := range append(append([]string{}, domain.MemberStates...), domain.OtherCountries...) {
		fmt.Fprintf(&b, "| %s |", cc)
		for i := range coverageColumns {
			v := strings.Join(cells[cc][i], ", ")
			if v == "" {
				v = "–"
			}
			b.WriteString(" " + v + " |")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n* off by default; on when the country is selected with `--countries`. IBANs, cards, VAT numbers, contact data and the other cross-border detectors cover every country.\n")
	return b.String()
}

func columnOf(c detectors.Concept) int {
	for i, col := range coverageColumns {
		if col.concept == c || col.concept == detectors.SocialSecurity && c == detectors.HealthInsurance || col.concept == detectors.IDCard && c == detectors.Passport {
			return i
		}
	}
	return -1
}

// DetectorTable is the Markdown reference in docs/DETECTORS.md, generated from the registry.
func DetectorTable() string {
	var b strings.Builder
	b.WriteString("# Detectors\n\nGenerated by `pii-eu detectors --markdown`; a test fails when this file is out of date.\n\n")
	b.WriteString("Base confidence is what a valid match gets on its own. A keyword on the same line or the line before raises it one step; a test or fixture path lowers it one step; a published sample value is always low. The default `--min-confidence` is medium.\n\n")
	b.WriteString("Keywords are the identifier's own names. Concepts add the shared words for that kind of data in all 24 EU languages, from `internal/detectors/keywords/`. Detectors marked off by default run when switched on in `[detectors]` or when their country is selected with `--countries`.\n\n")
	b.WriteString("| Name | Country | Identifier | How a match is confirmed | Base confidence | Keywords | Concepts |\n|---|---|---|---|---|---|---|\n")
	for _, d := range detectors.All() {
		kws := make([]string, 0, len(d.Keywords))
		for _, k := range d.Keywords {
			kws = append(kws, "`"+k+"`")
		}
		concepts := make([]string, 0, len(d.Concepts))
		for _, c := range d.Concepts {
			concepts = append(concepts, string(c))
		}
		label := d.Label
		if !d.OnByDefault {
			label += " (off by default)"
		}
		if d.NeedsKeyword {
			label += " (only with a keyword)"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | %s |\n", d.Name, d.Country, label, d.Validation, d.Base, strings.Join(kws, ", "), strings.Join(concepts, ", "))
	}
	return b.String()
}
