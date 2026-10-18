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
		code, err = runDetectors(stdout)
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
  pii-eu detectors             list what can be detected
  pii-eu version               print the version

Scan flags:
  --config FILE            settings file (default: <path>/.pii-eu.toml)
  --include GLOB           only scan matching paths (repeatable)
  --exclude GLOB           skip matching paths (repeatable)
  --min-confidence LEVEL   report low, medium or high and above (default: medium)
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
	return cfg, nil
}

func enabledDetectors(cfg config.Config) []detectors.Detector {
	var out []detectors.Detector
	for _, d := range detectors.All() {
		if cfg.Enabled(d.Name, d.OnByDefault) {
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
	findings = atLeast(findings, cfg.MinConfidence)
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
	for name := range cfg.Detectors {
		if _, ok := detectors.Get(name); !ok {
			return engine.Result{}, cfg, usageError{fmt.Sprintf("unknown detector %q in settings: run `pii-eu detectors`", name)}
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

func atLeast(fs []domain.Finding, min domain.Confidence) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Confidence >= min {
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

func runDetectors(stdout io.Writer) (int, error) {
	var b strings.Builder
	for _, d := range detectors.All() {
		state := ""
		if !d.OnByDefault {
			state = " (off by default)"
		}
		fmt.Fprintf(&b, "%-20s %-3s %s%s\n", d.Name, d.Country, d.Label, state)
	}
	_, err := io.WriteString(stdout, b.String())
	return ExitClean, err
}
