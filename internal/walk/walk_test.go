package walk

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestGlob(t *testing.T) {
	cases := []struct {
		pattern string
		yes     []string
		no      []string
	}{
		{"*.json", []string{"a.json", "x/y/a.json"}, []string{"a.json5", "a.jsonl"}},
		{"vendor", []string{"vendor/a.go", "x/vendor/b.go"}, []string{"vendors/a.go"}},
		{"docs/", []string{"docs/a.md", "docs/x/b.md"}, []string{"mydocs/a.md"}},
		{"/build", []string{"build/a"}, []string{"x/build/a"}},
		{"src/**/*.go", []string{"src/a.go", "src/x/y/a.go"}, []string{"lib/src/a.go", "src/a.js"}},
		{"test?.txt", []string{"test1.txt"}, []string{"test12.txt"}},
	}
	for _, c := range cases {
		g, err := compileGlob(c.pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range c.yes {
			if !g.re.MatchString(p) {
				t.Errorf("%q should match %q", c.pattern, p)
			}
		}
		for _, p := range c.no {
			if g.re.MatchString(p) {
				t.Errorf("%q should not match %q", c.pattern, p)
			}
		}
	}
	if _, err := compileGlob(""); !errors.Is(err, ErrBadGlob) {
		t.Fatal("empty pattern should fail")
	}
}

func tree(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func rels(fs []File) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Rel)
	}
	return out
}

func TestFilesWithoutGit(t *testing.T) {
	dir := tree(t, "a.txt", "src/b.go", "src/c.json", ".git/config", "node_modules/x.js")
	fs, _, err := Files(dir, Options{Exclude: []string{"node_modules"}, NoGit: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rels(fs), []string{"a.txt", "src/b.go", "src/c.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	fs, _, err = Files(dir, Options{Include: []string{"*.go"}, NoGit: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := rels(fs); !reflect.DeepEqual(got, []string{"src/b.go"}) {
		t.Fatalf("include: %v", got)
	}
}

func TestFilesHonoursGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := tree(t, "keep.txt", "ignored/secret.txt", ".gitignore")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	fs, _, err := Files(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := rels(fs); !reflect.DeepEqual(got, []string{".gitignore", "keep.txt"}) {
		t.Fatalf("got %v", got)
	}
}

func TestSingleFileAndMissingPath(t *testing.T) {
	dir := tree(t, "one.txt")
	fs, _, err := Files(filepath.Join(dir, "one.txt"), Options{})
	if err != nil || len(fs) != 1 {
		t.Fatalf("single file: %v %v", fs, err)
	}
	if _, _, err := Files(filepath.Join(dir, "missing"), Options{}); err == nil {
		t.Fatal("missing path should fail")
	}
}

func TestIsBinary(t *testing.T) {
	if IsBinary([]byte("hello\nworld")) || !IsBinary([]byte("a\x00b")) {
		t.Fatal("binary detection wrong")
	}
}

// TestUntrustedRepositoryConfigRunsNothing: a scanned folder's own .git/config must not make git run a
// command, here through core.fsmonitor, while files are listed or staged files are read (ADR 0007).
func TestUntrustedRepositoryConfigRunsNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the probe command is a POSIX shell command")
	}
	dir := tree(t, "a.txt")
	marker := filepath.Join(t.TempDir(), "ran")
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "core.fsmonitor", "touch '" + marker + "'; false"},
		{"add", "a.txt"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.fsmonitor=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	for _, opts := range []Options{{}, {Staged: true}} {
		fs, _, err := Files(dir, opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := rels(fs); !reflect.DeepEqual(got, []string{"a.txt"}) {
			t.Fatalf("staged %v: got %v", opts.Staged, got)
		}
		if opts.Staged {
			r, err := Open(fs[0])
			if err != nil {
				t.Fatal(err)
			}
			r.Close()
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("git ran the command from the scanned folder's core.fsmonitor")
	}
}
