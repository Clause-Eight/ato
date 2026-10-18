// Package walk lists the files to scan. Inside a git work tree it asks git, so .gitignore rules apply exactly as git applies them.
package walk

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// File is one file to scan. Rel uses forward slashes.
type File struct {
	Rel  string
	Abs  string
	Size int64
}

// Options select files.
type Options struct {
	Include     []string
	Exclude     []string
	MaxFileSize int64
	NoGit       bool
}

var alwaysSkippedDirs = map[string]bool{".git": true, ".hg": true, ".svn": true}

// Files lists the files under root that match the options, sorted by path.
func Files(root string, opts Options) ([]File, []domain.Skipped, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open %s: %w", root, err)
	}
	if !info.IsDir() {
		return single(root, info, opts)
	}
	rels, err := candidates(root, opts.NoGit)
	if err != nil {
		return nil, nil, err
	}
	inc, err := compileAll(opts.Include)
	if err != nil {
		return nil, nil, err
	}
	exc, err := compileAll(opts.Exclude)
	if err != nil {
		return nil, nil, err
	}
	var files []File
	var skipped []domain.Skipped
	for _, rel := range rels {
		if !selected(rel, inc, exc) {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		st, err := os.Lstat(abs)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		if opts.MaxFileSize > 0 && st.Size() > opts.MaxFileSize {
			skipped = append(skipped, domain.Skipped{Path: rel, Reason: fmt.Sprintf("larger than %d bytes", opts.MaxFileSize)})
			continue
		}
		files = append(files, File{Rel: rel, Abs: abs, Size: st.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, skipped, nil
}

func single(path string, info fs.FileInfo, opts Options) ([]File, []domain.Skipped, error) {
	rel := filepath.ToSlash(filepath.Base(path))
	if opts.MaxFileSize > 0 && info.Size() > opts.MaxFileSize {
		return nil, []domain.Skipped{{Path: rel, Reason: fmt.Sprintf("larger than %d bytes", opts.MaxFileSize)}}, nil
	}
	return []File{{Rel: filepath.ToSlash(path), Abs: path, Size: info.Size()}}, nil, nil
}

func selected(rel string, inc, exc []glob) bool {
	if len(inc) > 0 && !matchAny(inc, rel) {
		return false
	}
	return !matchAny(exc, rel)
}

func candidates(root string, noGit bool) ([]string, error) {
	if !noGit {
		if rels, ok := gitFiles(root); ok {
			return rels, nil
		}
	}
	return walkFiles(root)
}

func gitFiles(root string) ([]string, bool) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, false
	}
	top, err := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || strings.TrimSpace(string(top)) != "true" {
		return nil, false
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, false
	}
	var rels []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) > 0 {
			rels = append(rels, string(p))
		}
	}
	return rels, true
}

func walkFiles(root string) ([]string, error) {
	var rels []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if path != root && alwaysSkippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list %s: %w", root, err)
	}
	return rels, nil
}

// IsBinary reports whether the start of a file looks binary.
func IsBinary(head []byte) bool {
	if len(head) > 8000 {
		head = head[:8000]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// ErrBadGlob marks an invalid include or exclude pattern.
var ErrBadGlob = errors.New("invalid pattern")
