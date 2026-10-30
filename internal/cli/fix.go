package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/baseline"
	"github.com/Clause-Eight/ato/v2/internal/config"
	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/fix"
)

// runFix replaces the values a scan would report with synthetic ones. Only test, fixture and example paths
// change unless --all is given; --dry-run prints a diff with the found values masked instead.
func runFix(args []string, stdout, stderr io.Writer) (int, error) {
	f, root, err := parseFlags("fix", args, stderr, func(fs *flag.FlagSet, f *scanFlags) {
		fs.BoolVar(&f.dryRun, "dry-run", false, "")
		fs.BoolVar(&f.all, "all", false, "")
	})
	if err != nil {
		return 0, err
	}
	switch {
	case root == "-":
		return 0, usageError{"fix needs a folder or file, not stdin"}
	case f.staged:
		return 0, usageError{"fix changes the working copy, so it does not take --staged"}
	case f.showValues || f.output != "" || f.format != "text":
		return 0, usageError{"fix does not take --show-values, --format or --output"}
	}
	res, cfg, err := collect(f, root)
	if err != nil {
		return 0, err
	}
	findings, _ := withoutIgnored(res.Findings)
	b, err := baseline.Load(baselinePath(f, root), f.baselinePath != "")
	if err != nil {
		return 0, usageError{err.Error()}
	}
	findings, _ = b.Filter(findings)
	findings = toFix(findings, cfg)

	byPath := map[string][]domain.Finding{}
	var paths []string
	outside := 0
	for _, fd := range findings {
		if fd.Kind != domain.KindValue || fd.Sample {
			continue
		}
		if !fd.TestPath && !f.all {
			outside++
			continue
		}
		if byPath[fd.Location.Path] == nil {
			paths = append(paths, fd.Location.Path)
		}
		byPath[fd.Location.Path] = append(byPath[fd.Location.Path], fd)
	}
	everything := map[string][]domain.Finding{}
	for _, fd := range res.Findings {
		everything[fd.Location.Path] = append(everything[fd.Location.Path], fd)
	}

	var files []fix.File
	unreadable := len(res.Summary.Unreadable)
	for _, p := range paths {
		data, err := os.ReadFile(diskPath(root, p))
		if err != nil {
			fmt.Fprintf(stderr, "ato: could not read %s (%s)\n", p, err)
			unreadable++
			continue
		}
		files = append(files, fix.File{Path: p, Content: data, Replace: byPath[p], All: everything[p]})
	}

	var replaced, changedFiles, unmarked, stale, unfixable int
	for _, o := range fix.Run(files) {
		stale += o.Stale
		unfixable += o.Unfixable
		if len(o.Changes) == 0 {
			continue
		}
		replaced += len(o.Changes)
		changedFiles++
		for _, c := range o.Changes {
			if !c.Replacement.Marked {
				unmarked++
			}
		}
		if f.dryRun {
			fmt.Fprint(stdout, o.Diff)
			continue
		}
		if err := writeKeepingMode(diskPath(root, o.Path), o.Content); err != nil {
			return 0, fmt.Errorf("cannot write %s: %w", o.Path, err)
		}
		fmt.Fprintf(stdout, "%s: replaced %d %s\n", o.Path, len(o.Changes), plural(len(o.Changes), "value", "values"))
	}
	fmt.Fprintln(stdout, fixSummary(f.dryRun, replaced, changedFiles, outside, unmarked, stale, unfixable))
	if unreadable > 0 {
		return ExitIncomplete, nil
	}
	return ExitClean, nil
}

func fixSummary(dryRun bool, replaced, files, outside, unmarked, stale, unfixable int) string {
	var lines []string
	switch {
	case replaced == 0:
		lines = append(lines, "Nothing to fix.")
	case dryRun:
		lines = append(lines, fmt.Sprintf("Would replace %d %s in %d %s.", replaced, plural(replaced, "value", "values"), files, plural(files, "file", "files")))
	default:
		lines = append(lines, fmt.Sprintf("Replaced %d %s in %d %s with synthetic ones.", replaced, plural(replaced, "value", "values"), files, plural(files, "file", "files")))
	}
	if outside > 0 {
		lines = append(lines, fmt.Sprintf("%d %s outside test, fixture and example paths left alone; pass --all to replace them too.", outside, plural(outside, "value", "values")))
	}
	if unmarked > 0 {
		lines = append(lines, fmt.Sprintf("%d %s could not be given the synthetic mark, so a scan may still report %s.", unmarked, plural(unmarked, "replacement", "replacements"), plural(unmarked, "it", "them")))
	}
	if unfixable > 0 {
		lines = append(lines, fmt.Sprintf("%d %s no synthetic replacement and %s left alone.", unfixable, plural(unfixable, "value has", "values have"), plural(unfixable, "was", "were")))
	}
	if stale > 0 {
		lines = append(lines, fmt.Sprintf("%d %s no longer where the scan found %s and %s left alone.", stale, plural(stale, "value was", "values were"), plural(stale, "it", "them"), plural(stale, "was", "were")))
	}
	return strings.Join(lines, "\n")
}

// diskPath turns a finding's path back into a path on disk. A file given directly keeps the path it was given.
func diskPath(root, rel string) string {
	if st, err := os.Stat(root); err == nil && !st.IsDir() {
		return root
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

func writeKeepingMode(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}

// toFix keeps what a scan would report, and also any value whose own keyword is next to it whatever its
// confidence: a CNP labelled "cnp" in a fixture is real data even when its context lowered it.
func toFix(fs []domain.Finding, cfg config.Config) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Confidence >= cfg.MinConfidenceFor(f.Detector) || f.Kind == domain.KindValue && f.Keyword {
			out = append(out, f)
		}
	}
	return out
}
