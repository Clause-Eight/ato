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
  pii-eu scan [flags] [path]   scan a folder or file (default: current folder)
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
	case "no-git", "show-values":
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
	cfg, err := loadConfig(f, root)
	if err != nil {
		return 0, err
	}
	for name := range cfg.Detectors {
		if _, ok := detectors.Get(name); !ok {
			return 0, usageError{fmt.Sprintf("unknown detector %q in settings: run `pii-eu detectors`", name)}
		}
	}
	res, err := engine.Scan(engine.Request{
		Root:      root,
		Walk:      walk.Options{Include: cfg.Include, Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize, NoGit: f.noGit},
		Detectors: enabledDetectors(cfg),
	})
	if err != nil {
		if errors.Is(err, walk.ErrBadGlob) {
			return 0, usageError{err.Error()}
		}
		return 0, err
	}
	findings := atLeast(res.Findings, cfg.MinConfidence)
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
