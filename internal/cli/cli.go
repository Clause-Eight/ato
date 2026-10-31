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

	"github.com/Clause-Eight/ato/v2/internal/art9"
	"github.com/Clause-Eight/ato/v2/internal/baseline"
	"github.com/Clause-Eight/ato/v2/internal/config"
	"github.com/Clause-Eight/ato/v2/internal/datamap"
	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/engine"
	"github.com/Clause-Eight/ato/v2/internal/report"
	"github.com/Clause-Eight/ato/v2/internal/walk"
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
	case "inventory":
		code, err = runInventory(args[1:], stdout, stderr)
	case "map":
		code, err = runMap(args[1:], stdout, stderr)
	case "fix":
		code, err = runFix(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "ato %s\n", Version)
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
	fmt.Fprintf(stderr, "ato: %s\n", err)
	var ue usageError
	if errors.As(err, &ue) {
		fmt.Fprintln(stderr, "Run `ato help` for usage.")
	}
	return ExitUsage
}

const usage = `ATO, Clause Eight's EU personal-data scanner: finds EU personal data in code, fixtures and logs.

Usage:
  ato scan [flags] [path]      scan a folder or file (default: current folder); "-" reads stdin
  ato scan --image FILE        scan a container image saved with ` + "`docker save`" + `, layer by layer
  ato baseline [path]          accept current findings so only new ones are reported
  ato inventory [flags] [path]  list the personal-data fields in schemas and models, and the
                               data stores declared in Terraform
  ato map [flags] [path]       draft a data map: stores, logged personal data and third-party recipients
  ato fix [flags] [path]       replace found values in test, fixture and example files with synthetic ones
  ato detectors [--markdown]   list what can be detected
  ato version                  print the version

Scan flags:
  --config FILE            settings file (default: <path>/.ato.toml)
  --include GLOB           only scan matching paths (repeatable)
  --exclude GLOB           skip matching paths (repeatable)
  --min-confidence LEVEL   report low, medium or high and above (default: medium)
  --countries LIST         only run detectors for these countries, e.g. ro,bg or a preset: cee, dach,
                           benelux, nordics, baltics, eu, eea (default: every EU country)
  --format FORMAT          text, json, sarif, github, codequality or html (default: text)
  --output FILE            write the report to a file instead of stdout
  --no-git                 walk the folder instead of asking git for the file list
  --staged                 scan the staged version of staged files (for pre-commit hooks)
  --baseline FILE          baseline file (default: <path>/.ato-baseline.json)
  --show-values            print found values in full in text output (never in files)
  --max-rows N             read at most N rows of each CSV, TSV or JSON Lines file; 0 reads every row
  --allow-partial          exit 0 or 1 even when a file was not read in full (cut at max_rows, over a
                           size limit, or a document format ATO does not open); see exit codes
                           (default: 100000)
  --image FILE             scan a saved container image (docker save or OCI layout tar) instead of a path

Archives (.gz, .tar, .tar.gz, .tgz) are scanned inside; findings name the entry, as in
dump.tar.gz!/data/users.csv. Limits against archive bombs are settings: max_archive_bytes,
max_archive_entries and max_archive_depth.

Inventory flags: --config, --include, --exclude, --no-git, --output as for scan, and
  --format FORMAT          json, csv or html (default: csv)

Map flags: --config, --include, --exclude, --no-git, --output as for scan, and
  --format FORMAT          markdown, json or html (default: markdown)
  --diff FILE              compare with a map saved as JSON; list what is new and exit 1 if anything is

Fix flags: --config, --include, --exclude, --min-confidence, --countries, --no-git, --baseline as for scan, and
  --dry-run                print the changes as a diff with found values masked, and change nothing
  --all                    also replace values outside test, fixture and example paths

Exit codes: 0 nothing found, 1 findings, 2 usage or settings error, 3 some files could not be read.
`

// deprecatedFlags maps a deprecated flag to what replaces it. A deprecated flag keeps working and prints a
// warning; it is removed no earlier than the next major release (docs/COMPATIBILITY.md).
var deprecatedFlags = map[string]string{}

func warnDeprecated(args []string, stderr io.Writer) {
	for _, a := range args {
		name := strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-")
		if repl, ok := deprecatedFlags[name]; ok && strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "ato: --%s is deprecated and will be removed in the next major version; use %s\n", name, repl)
		}
	}
}

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
	inventory     bool
	recipients    bool
	lifecycle     bool
	diff          string
	maxRows       int
	allowPartial  bool
	image         string
	dryRun        bool
	all           bool
	// stderr receives deprecation warnings.
	stderr io.Writer
}

func parseScanFlags(args []string, stderr io.Writer) (scanFlags, string, error) {
	return parseFlags("scan", args, stderr, nil)
}

// parseFlags reads the scan flags, plus any that extra adds for one command.
func parseFlags(command string, args []string, stderr io.Writer, extra func(*flag.FlagSet, *scanFlags)) (scanFlags, string, error) {
	f := scanFlags{stderr: stderr}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
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
	fs.StringVar(&f.diff, "diff", "", "")
	fs.IntVar(&f.maxRows, "max-rows", -1, "")
	fs.BoolVar(&f.allowPartial, "allow-partial", false, "")
	fs.StringVar(&f.image, "image", "", "")
	if extra != nil {
		extra(fs, &f)
	}
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	warnDeprecated(args, stderr)
	if err := fs.Parse(reorder(args)); err != nil {
		return f, "", err
	}
	if f.maxRows < -1 {
		return f, "", usageError{"--max-rows must be zero (every row) or more"}
	}
	switch {
	case f.image != "" && fs.NArg() > 0:
		return f, "", usageError{"give either a path or --image, not both"}
	case f.image != "":
		return f, f.image, nil
	case fs.NArg() == 0:
		return f, ".", nil
	case fs.NArg() == 1:
		return f, fs.Arg(0), nil
	}
	return f, "", usageError{command + " takes one path"}
}

// reorder moves flags in front of the path so `ato scan . --format json` works.
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
	case "no-git", "show-values", "staged", "dry-run", "all", "allow-partial":
		return false
	}
	return true
}

func loadConfig(f scanFlags, root string) (config.Config, error) {
	path, explicit := f.configPath, true
	if path == "" {
		explicit = false
		path = defaultFile(scanDir(root), config.FileName, config.LegacyFileName, f.stderr)
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
	if f.maxRows >= 0 {
		cfg.MaxRows = f.maxRows
	}
	cfg.AllowPartial = cfg.AllowPartial || f.allowPartial
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

func enabledInfra(cfg config.Config) []string {
	var out []string
	for _, name := range engine.InfraDetectors() {
		if cfg.Enabled(name, []string{domain.CrossBorder}, true) {
			out = append(out, name)
		}
	}
	return out
}

func enabledRecords(cfg config.Config) []string {
	var out []string
	for _, name := range engine.RecordDetectors() {
		if cfg.Enabled(name, []string{domain.CrossBorder}, true) {
			out = append(out, name)
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
	if err := writeReport(f, report.Report{Tool: "ato", Version: Version, Root: root, Findings: findings, Summary: res.Summary}, stdout); err != nil {
		return 0, err
	}
	switch {
	case len(findings) > 0:
		return ExitFindings, nil
	case incomplete(res.Summary, cfg):
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
			if _, ok := detectors.Get(name); !ok && !engineDetector[name] {
				return engine.Result{}, cfg, usageError{fmt.Sprintf("unknown detector %q in settings: run `ato detectors`", name)}
			}
		}
	}
	dets := enabledDetectors(cfg)
	erasure := cfg.Enabled(engine.NoErasureDetector, []string{domain.CrossBorder}, false)
	retention := cfg.Enabled(engine.NoRetentionDetector, []string{domain.CrossBorder}, false)
	if root == "-" {
		fs, err := engine.ScanReader("<stdin>", Stdin, dets)
		warnLegacyIgnore(fs, f.stderr)
		return engine.Result{Findings: fs, Summary: domain.Summary{FilesScanned: 1}}, cfg, err
	}
	req := engine.Request{
		Root: root,
		Walk: walk.Options{Include: cfg.Include, Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize, NoGit: f.noGit, Staged: f.staged,
			Limits:       walk.Limits{MaxBytes: cfg.MaxArchiveBytes, MaxEntries: cfg.MaxArchiveEntries, MaxDepth: cfg.MaxArchiveDepth},
			ImageExclude: cfg.ImageExclude},
		Detectors:         dets,
		SpecialCategories: cfg.Enabled(engine.SpecialCategoryDetector, []string{domain.CrossBorder}, true),
		Sinks:             cfg.Enabled(engine.SinkDetector, []string{domain.CrossBorder}, true),
		Inventory:         f.inventory,
		Recipients:        f.recipients,
		Lifecycle:         f.lifecycle || erasure || retention,
		ErasureFindings:   erasure,
		RetentionFindings: retention,
		Infra:             enabledInfra(cfg),
		Records:           enabledRecords(cfg),
		MaxRows:           cfg.MaxRows,
	}
	scan := engine.Scan
	if f.image != "" {
		if f.staged {
			return engine.Result{}, cfg, usageError{"--staged does not apply to --image"}
		}
		scan = engine.ScanImage
	}
	res, err := scan(req)
	warnLegacyIgnore(res.Findings, f.stderr)
	if errors.Is(err, walk.ErrBadGlob) || errors.Is(err, walk.ErrNotGit) || errors.Is(err, walk.ErrNotImage) {
		return res, cfg, usageError{err.Error()}
	}
	return res, cfg, err
}

// engineDetector names the findings made by the engine rather than by a registered detector. Settings may switch
// them on or off like a detector.
var engineDetector = func() map[string]bool {
	m := map[string]bool{engine.SpecialCategoryDetector: true, engine.SinkDetector: true, engine.NoErasureDetector: true, engine.NoRetentionDetector: true}
	for _, name := range engine.InfraDetectors() {
		m[name] = true
	}
	for _, name := range engine.RecordDetectors() {
		m[name] = true
	}
	return m
}()

// Stdin is read by `ato scan -`. Tests replace it.
var Stdin io.Reader = os.Stdin

func baselinePath(f scanFlags, root string) string {
	if f.baselinePath != "" {
		return f.baselinePath
	}
	return defaultFile(scanDir(root), baseline.FileName, baseline.LegacyFileName, f.stderr)
}

// defaultFile is name in dir, or the deprecated legacy name when only that file exists, with a warning. The
// legacy names are read in 2.x and dropped in the next major version (docs/COMPATIBILITY.md).
func defaultFile(dir, name, legacy string, stderr io.Writer) string {
	path := filepath.Join(dir, name)
	old := filepath.Join(dir, legacy)
	if exists(path) || !exists(old) {
		return path
	}
	if stderr != nil {
		fmt.Fprintf(stderr, "ato: reading %s; the name %s is deprecated, rename it to %s\n", old, legacy, name)
	}
	return old
}

// scanDir is the folder that holds the settings and baseline files for a scan of root.
func scanDir(root string) string {
	if st, err := os.Stat(root); err == nil && !st.IsDir() {
		return filepath.Dir(root)
	}
	return root
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// warnLegacyIgnore prints one warning when ignore comments use the deprecated pii-eu:ignore form.
func warnLegacyIgnore(fs []domain.Finding, stderr io.Writer) {
	n, first := 0, ""
	for _, f := range fs {
		if f.LegacyIgnore {
			if n == 0 {
				first = f.Location.Path
			}
			n++
		}
	}
	if n > 0 && stderr != nil {
		fmt.Fprintf(stderr, "ato: %d %s use the deprecated pii-eu:ignore comment (first in %s); write ato:ignore instead\n", n, plural(n, "finding", "findings"), first)
	}
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
	path := f.baselinePath
	if path == "" {
		path = filepath.Join(scanDir(root), baseline.FileName)
		if old := filepath.Join(scanDir(root), baseline.LegacyFileName); exists(old) {
			fmt.Fprintf(stderr, "ato: writing %s; delete the deprecated %s, which is read only when %s is absent\n", path, old, baseline.FileName)
		}
	}
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
	for _, c := range domain.InfraChecks {
		fmt.Fprintf(&b, "%-16s %-3s %-7s %s (Terraform)\n", c.Detector, domain.CrossBorder, domain.Medium, c.Label)
	}
	for _, c := range art9.Checks {
		fmt.Fprintf(&b, "%-16s %-3s %-7s %s\n", c.Detector, domain.CrossBorder, domain.High, c.Label)
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
		if d.Country == domain.CrossBorder || d.Special != "" {
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
	b.WriteString("# Detectors\n\nGenerated by `ato detectors --markdown`; a test fails when this file is out of date.\n\n")
	b.WriteString("Base confidence is what a valid match gets on its own. A keyword on the same line or the line before raises it one step; a test or fixture path lowers it one step; a published sample value is always low. The default `--min-confidence` is medium.\n\n")
	b.WriteString("Keywords are the identifier's own names. Concepts add the shared words for that kind of data in all 24 EU languages, from `internal/detectors/keywords/`. Detectors marked off by default run when switched on in `[detectors]` or when their country is selected with `--countries`.\n\n")
	b.WriteString("Schema and model fields whose names say they hold special-category data (GDPR Art. 9: health, genetic, biometric, racial or ethnic origin, political opinions, religious or philosophical beliefs, trade union membership, sex life or sexual orientation) are reported as `special-category` findings at medium confidence. Switch them off with `special-category = false` under `[detectors]`.\n\n")
	b.WriteString("Tables and models holding personal data can also be checked for an erasure path (GDPR Art. 17: a delete, an anonymising update or a cascade from an erasable parent) and a retention limit (Art. 5(1)(e): a time-based delete, a TTL or a partition drop). Switch the `no-erasure-path` and `no-retention` findings on under `[detectors]`; they are off by default because data can be removed in ways the code does not show. `ato map` always shows both.\n\n")
	b.WriteString("Log, print, error-tracker and analytics calls in Go, Python, JavaScript and TypeScript, Java, Kotlin, Ruby and PHP that are passed a field whose name says it holds personal data (`user.email`), a whole request body (`req.body`) or a user-like object (`currentUser`) are reported as `log-sink` findings, with the sink (log, error-tracker or analytics) and the data category: medium confidence for a named field or a request body, low for a whole object or an online identifier. Switch them off with `log-sink = false` under `[detectors]`.\n\n")
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
		if d.NeedsPerson {
			label += " (only in a record about a person)"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | %s |\n", d.Name, d.Country, label, d.Validation, d.Base, strings.Join(kws, ", "), strings.Join(concepts, ", "))
	}
	b.WriteString(recordTable())
	b.WriteString(storeTable())
	return b.String()
}

// recordTable is the part of docs/DETECTORS.md about records holding Art. 9 data.
func recordTable() string {
	var b strings.Builder
	b.WriteString("\n## Health, genetic and biometric records\n\n")
	b.WriteString("`ato scan` also reads the shape of whole records and files that hold special-category data about a person (docs/adr/0013-art9-records.md) and reports them as `record` findings with the Art. 9 category. They start high; a test or fixture path lowers them one step, and a record about a placeholder (a specification's example patient, a generated name with digits, the public reference genomes) is low. Switch a check off under `[detectors]`, for example `rsid-genotypes = false`.\n\n")
	b.WriteString("| Name | Finding | Reported when |\n|---|---|---|\n")
	for _, c := range art9.Checks {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", c.Detector, c.Label, c.When)
	}
	return b.String()
}

// storeTable is the part of docs/DETECTORS.md about Terraform data stores.
func storeTable() string {
	var b strings.Builder
	b.WriteString("\n## Terraform data stores\n\n")
	b.WriteString("`ato scan` reads the data stores declared in `.tf` files, without running Terraform, and reports these checks as `infra` findings. A finding is high when the store is linked to personal data (a tag such as `data_classification = \"pii\"` or `pii = true`, or a name that matches a table or model in the inventory) and medium otherwise. A setting that comes from something the code does not show, such as a variable without a default, is never read as a failure: it is reported at low confidence, and only for linked stores. Switch a check off under `[detectors]`, for example `tf-no-retention = false`.\n\n")
	b.WriteString("| Name | Finding | Reported when | Supports |\n|---|---|---|---|\n")
	for _, c := range domain.InfraChecks {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", c.Detector, c.Label, c.When, c.Rule)
	}
	b.WriteString("\nResource types read as data stores:\n\n")
	types := engine.StoreTypes()
	for _, p := range []struct{ key, title string }{{"aws", "AWS"}, {"google", "Google Cloud"}, {"azurerm", "Azure"}} {
		names := make([]string, 0, len(types[p.key]))
		for _, t := range types[p.key] {
			names = append(names, "`"+t+"`")
		}
		fmt.Fprintf(&b, "- %s: %s\n", p.title, strings.Join(names, ", "))
	}
	b.WriteString("\nModule calls: a local module (`source = \"./db\"`) whose folder is scanned is read as its own folder. Any other `module` block (registry, git or a local folder outside the scan) is not read inside. When its source or label names a kind of store (`terraform-aws-modules/rds/aws`, `patients_db`) or it passes arguments that look like store settings (`storage_encrypted`, `retention_in_days`, `kms_key_id`, `publicly_accessible`), it is listed in the inventory's `stores` as type `module`, with its source, those arguments by name and every setting unknown. A module call gives no scan findings.\n")
	return b.String()
}

func runInventory(args []string, stdout, stderr io.Writer) (int, error) {
	f, root, err := parseScanFlags(args, stderr)
	if err != nil {
		return 0, err
	}
	if root == "-" {
		return 0, usageError{"an inventory needs a folder or file, not stdin"}
	}
	if f.format == "text" {
		f.format = "csv"
	}
	if !contains(report.InventoryFormats, f.format) {
		return 0, usageError{fmt.Sprintf("unknown inventory format %q: use json, csv or html", f.format)}
	}
	f.inventory = true
	res, cfg, err := collect(f, root)
	if err != nil {
		return 0, err
	}
	inv := report.Inventory{Tool: "ato", Version: Version, Entries: res.Inventory, Stores: res.Stores, Files: res.Summary.FilesScanned}
	w := stdout
	if f.output != "" {
		file, err := os.Create(f.output)
		if err != nil {
			return 0, fmt.Errorf("cannot write %s: %w", f.output, err)
		}
		defer file.Close()
		w = file
	}
	if err := report.WriteInventory(f.format, w, inv); err != nil {
		return 0, err
	}
	if incomplete(res.Summary, cfg) {
		return ExitIncomplete, nil
	}
	return ExitClean, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func runMap(args []string, stdout, stderr io.Writer) (int, error) {
	f, root, err := parseScanFlags(args, stderr)
	if err != nil {
		return 0, err
	}
	if root == "-" {
		return 0, usageError{"a data map needs a folder or file, not stdin"}
	}
	if f.format == "text" {
		f.format = "markdown"
	}
	if !contains(report.MapFormats, f.format) {
		return 0, usageError{fmt.Sprintf("unknown map format %q: use markdown, json or html", f.format)}
	}
	var previous *datamap.Map
	if f.diff != "" {
		file, err := os.Open(f.diff)
		if err != nil {
			return 0, usageError{fmt.Sprintf("cannot read %s: %v", f.diff, err)}
		}
		m, err := report.ReadMap(file)
		file.Close()
		if err != nil {
			return 0, usageError{fmt.Sprintf("%s: %v", f.diff, err)}
		}
		previous = &m
	}
	f.inventory, f.recipients, f.lifecycle = true, true, true
	res, cfg, err := collect(f, root)
	if err != nil {
		return 0, err
	}
	findings, _ := withoutIgnored(res.Findings)
	m := datamap.Build(datamap.Input{Inventory: res.Inventory, Findings: findings, Recipients: res.Recipients, Lifecycle: res.Lifecycle, Infra: res.Stores})
	if previous != nil {
		changes := datamap.Diff(*previous, m)
		dm := report.DataMap{Tool: "ato", Version: Version, Map: m, Previous: previous, PreviousName: filepath.Base(f.diff)}
		switch {
		case f.output != "":
			if err := writeMapFile(f, dm); err != nil {
				return 0, err
			}
		case f.format == "html":
			// The page carries the comparison, so it replaces the text summary.
			if err := report.WriteMap(f.format, stdout, dm); err != nil {
				return 0, err
			}
			if len(changes) > 0 {
				return ExitFindings, nil
			}
			return ExitClean, nil
		}
		if len(changes) == 0 {
			fmt.Fprintln(stdout, "Nothing new since the previous map.")
			return ExitClean, nil
		}
		fmt.Fprintf(stdout, "New since the previous map (%d):\n", len(changes))
		for _, c := range changes {
			fmt.Fprintf(stdout, "  %-18s %-17s %s\n", c.Component, c.Kind, c.Name)
		}
		return ExitFindings, nil
	}
	dm := report.DataMap{Tool: "ato", Version: Version, Map: m}
	if f.output != "" {
		if err := writeMapFile(f, dm); err != nil {
			return 0, err
		}
	} else if err := report.WriteMap(f.format, stdout, dm); err != nil {
		return 0, err
	}
	if incomplete(res.Summary, cfg) {
		return ExitIncomplete, nil
	}
	return ExitClean, nil
}

func writeMapFile(f scanFlags, m report.DataMap) error {
	file, err := os.Create(f.output)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", f.output, err)
	}
	if err := report.WriteMap(f.format, file, m); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// incomplete reports whether a scan must exit 3: a file could not be read, or one that may hold personal data was
// not read in full and allow_partial is off (ADR 8).
func incomplete(s domain.Summary, cfg config.Config) bool {
	return len(s.Unreadable) > 0 || !cfg.AllowPartial && len(report.NotRead(s)) > 0
}
