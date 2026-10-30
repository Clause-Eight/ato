package walk

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type tarMember struct {
	name string
	body []byte
}

func tarOf(t *testing.T, members ...tarMember) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, m := range members {
		if strings.HasSuffix(m.name, "/") {
			if err := tw.WriteHeader(&tar.Header{Name: m.name, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := tw.WriteHeader(&tar.Header{Name: m.name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(m.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(m.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gz(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// visit reads an archive and returns what each entry held, keyed by its report path.
func visit(t *testing.T, rel string, data []byte, opts Options) (map[string]string, Outcome) {
	t.Helper()
	got := map[string]string{}
	out, err := VisitArchive(rel, bytes.NewReader(data), opts, func(e Entry, r io.Reader) error {
		body, err := io.ReadAll(r)
		got[e.Rel] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, out
}

func keysOf(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestArchivesAreReadInside(t *testing.T) {
	inner := tarOf(t, tarMember{"notes.txt", []byte("inner\n")})
	data := gz(t, tarOf(t,
		tarMember{"./data/", nil},
		tarMember{"./data/users.csv", []byte("a,b\n1,2\n")},
		tarMember{"logs/app.log.gz", gz(t, []byte("log line\n"))},
		tarMember{"nested/more.tar", inner},
		tarMember{"vendor/skip.txt", []byte("x\n")},
	))
	got, out := visit(t, "dump.tar.gz", data, Options{Exclude: []string{"vendor"}})
	want := []string{"dump.tar.gz!/data/users.csv", "dump.tar.gz!/logs/app.log.gz!/app.log", "dump.tar.gz!/nested/more.tar!/notes.txt"}
	if fmt.Sprint(keysOf(got)) != fmt.Sprint(want) || got[want[1]] != "log line\n" {
		t.Fatalf("got %v", got)
	}
	if len(out.Skipped)+len(out.Unreadable) != 0 {
		t.Fatalf("outcome %+v", out)
	}
	got, _ = visit(t, "users.csv.gz", gz(t, []byte("a\n")), Options{})
	if got["users.csv.gz!/users.csv"] != "a\n" {
		t.Fatalf("single gzip file: %v", got)
	}
	got, _ = visit(t, "backup.tgz", gz(t, tarOf(t, tarMember{"x.txt", []byte("x")})), Options{})
	if _, ok := got["backup.tgz!/x.txt"]; !ok {
		t.Fatalf("tgz: %v", got)
	}
}

func TestArchiveEntriesFollowTheSizeLimit(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 200)
	data := tarOf(t, tarMember{"big.txt", big}, tarMember{"big.csv", big}, tarMember{"small.txt", []byte("s")}, tarMember{"big.txt.gz", gz(t, big)})
	got, out := visit(t, "a.tar", data, Options{MaxFileSize: 100})
	if fmt.Sprint(keysOf(got)) != "[a.tar!/big.csv a.tar!/small.txt]" || len(out.Skipped) != 2 {
		t.Fatalf("got %v, %+v", keysOf(got), out)
	}
}

func TestArchiveBombsStopAtTheLimits(t *testing.T) {
	zeros := gz(t, make([]byte, 8<<20))
	got, out := visit(t, "bomb.gz", zeros, Options{Limits: Limits{MaxBytes: 1 << 20}})
	if len(got["bomb.gz!/bomb"]) > 2<<20 || len(out.Unreadable) != 1 || !strings.Contains(out.Unreadable[0].Reason, "max_archive_bytes") {
		t.Fatalf("bytes: %v %+v", keysOf(got), out)
	}

	var many []tarMember
	for i := range 50 {
		many = append(many, tarMember{fmt.Sprintf("f%02d.txt", i), []byte("x")})
	}
	got, out = visit(t, "many.tar", tarOf(t, many...), Options{Limits: Limits{MaxEntries: 10}})
	if len(got) != 10 || len(out.Unreadable) != 1 || !strings.Contains(out.Unreadable[0].Reason, "max_archive_entries") {
		t.Fatalf("entries: %d %+v", len(got), out)
	}

	level := tarOf(t, tarMember{"deep.txt", []byte("x")})
	for i := range 5 {
		level = gz(t, tarOf(t, tarMember{fmt.Sprintf("l%d.tar.gz", i), level}, tarMember{"top.txt", []byte("t")}))
	}
	got, out = visit(t, "deep.tar.gz", level, Options{Limits: Limits{MaxDepth: 3}})
	if len(got) != 3 || len(out.Skipped) != 1 || !strings.Contains(out.Skipped[0].Reason, "max_archive_depth") {
		t.Fatalf("depth: %v %+v", keysOf(got), out)
	}

	inner := tarOf(t, tarMember{"x.txt", bytes.Repeat([]byte("x"), 1<<20)})
	quine := gz(t, tarOf(t, tarMember{"a.tar", inner}, tarMember{"b.tar", inner}, tarMember{"c.tar.gz", gz(t, inner)}))
	if _, out := visit(t, "q.tar.gz", quine, Options{Limits: Limits{MaxBytes: 2 << 20}}); len(out.Unreadable) != 1 {
		t.Fatalf("nested archives share one budget: %+v", out)
	}
}

func TestBrokenArchivesAreUnreadable(t *testing.T) {
	_, out := visit(t, "x.gz", []byte("not gzip"), Options{})
	if len(out.Unreadable) != 1 || !strings.Contains(out.Unreadable[0].Reason, "gzip") {
		t.Fatalf("%+v", out)
	}
	good := tarOf(t, tarMember{"a.txt", []byte("a")}, tarMember{"b.txt", []byte("b")})
	got, out := visit(t, "cut.tar", good[:1100], Options{})
	if len(out.Unreadable) != 1 || got["cut.tar!/a.txt"] != "a" {
		t.Fatalf("a truncated tar keeps what was read: %v %+v", got, out)
	}
}

func TestArchivesAndDataFilesIgnoreTheFileSizeLimit(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("1,2\n"), 100)
	for _, name := range []string{"a.txt", "b.csv", "c.tar.gz", "d.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), big, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, skipped, err := Files(dir, Options{NoGit: true, MaxFileSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Rel)
	}
	if fmt.Sprint(names) != "[b.csv c.tar.gz d.jsonl]" || len(skipped) != 1 || skipped[0].Path != "a.txt" {
		t.Fatalf("files %v skipped %+v", names, skipped)
	}
}

// saveImage writes a synthetic `docker save` tarball. Each layer is a list of members; gzip compresses the
// layers. With oci the layout is index.json and blobs instead of manifest.json.
func saveImage(t *testing.T, oci, compress bool, layers ...[]tarMember) string {
	t.Helper()
	var top []tarMember
	var names []string
	var descs []map[string]string
	for i, l := range layers {
		body := tarOf(t, l...)
		if compress {
			body = gz(t, body)
		}
		hex := strings.Repeat(fmt.Sprintf("%x", i+10), 64)[:64]
		name := hex + "/layer.tar"
		if oci {
			name = "blobs/sha256/" + hex
		}
		names = append(names, name)
		descs = append(descs, map[string]string{"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip", "digest": "sha256:" + hex})
		top = append(top, tarMember{name, body})
	}
	config := []byte(`{"config":{"Env":["PATH=/usr/bin"]},"history":[]}`)
	cfgHex := strings.Repeat("c", 64)
	if oci {
		manifest, _ := json.Marshal(map[string]any{"config": map[string]string{"digest": "sha256:" + cfgHex}, "layers": descs})
		manHex := strings.Repeat("d", 64)
		index, _ := json.Marshal(map[string]any{"manifests": []map[string]string{
			{"mediaType": "application/vnd.oci.image.manifest.v1+json", "digest": "sha256:" + manHex},
			{"mediaType": "application/vnd.oci.image.manifest.v1+json", "digest": "sha256:" + strings.Repeat("e", 64)},
		}})
		top = append(top, tarMember{"blobs/sha256/" + cfgHex, config}, tarMember{"blobs/sha256/" + manHex, manifest}, tarMember{"index.json", index}, tarMember{"oci-layout", []byte(`{"imageLayoutVersion":"1.0.0"}`)})
	} else {
		manifest, _ := json.Marshal([]map[string]any{{"Config": cfgHex + ".json", "RepoTags": []string{"app:test"}, "Layers": names}})
		top = append(top, tarMember{cfgHex + ".json", config}, tarMember{"manifest.json", manifest})
	}
	p := filepath.Join(t.TempDir(), "image.tar")
	if err := os.WriteFile(p, tarOf(t, top...), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImagesAreReadLayerByLayer(t *testing.T) {
	base := []tarMember{{"etc/", nil}, {"etc/app/config.yml", []byte("v1\n")}, {"usr/share/doc/x.txt", []byte("doc\n")}, {"app/old.txt", []byte("old\n")}, {"srv/data/a.txt", []byte("a\n")}}
	top := []tarMember{{"etc/app/config.yml", []byte("v2\n")}, {"app/.wh.old.txt", nil}, {"srv/data/.wh..wh..opq", nil}, {"srv/keep.txt", []byte("k\n")}}
	for _, oci := range []bool{false, true} {
		p := saveImage(t, oci, oci, base, top)
		img, err := OpenImage(p, Options{ImageExclude: []string{"/usr/share/"}, Exclude: []string{"*.md"}})
		if err != nil {
			t.Fatalf("oci=%v: %v", oci, err)
		}
		if len(img.Layers) != 2 || len(img.Configs) != 1 || img.Layers[0].ID != "aaaaaaaaaaaa" {
			t.Fatalf("oci=%v: layers %+v configs %v", oci, img.Layers, img.Configs)
		}
		got := map[string]string{}
		for i := range img.Layers {
			out := img.VisitLayer(i, func(e Entry, r io.Reader) error {
				body, _ := io.ReadAll(r)
				got[e.Rel] = string(body)
				return nil
			})
			if len(out.Skipped)+len(out.Unreadable) != 0 {
				t.Fatalf("oci=%v: %+v", oci, out)
			}
		}
		prefix := filepath.ToSlash(filepath.Clean(p)) + "!/"
		if got[prefix+"aaaaaaaaaaaa/etc/app/config.yml"] != "v1\n" || got[prefix+"bbbbbbbbbbbb/etc/app/config.yml"] != "v2\n" || len(got) != 5 {
			t.Fatalf("oci=%v: %v", oci, keysOf(got))
		}
		for name, want := range map[string]string{
			"etc/app/config.yml": "replaced in later layer bbbbbbbbbbbb",
			"app/old.txt":        "deleted in later layer bbbbbbbbbbbb",
			"srv/data/a.txt":     "hidden by later layer bbbbbbbbbbbb",
		} {
			if why := img.Shadowed(0, name); !strings.HasPrefix(why, want) {
				t.Errorf("oci=%v %s: %q", oci, name, why)
			}
		}
		if why := img.Shadowed(1, "etc/app/config.yml"); why != "" {
			t.Errorf("the top layer is visible: %q", why)
		}
		var cfg string
		if err := img.VisitConfig(img.Configs[0], func(e Entry, r io.Reader) error {
			b, _ := io.ReadAll(r)
			cfg = string(b)
			return nil
		}); err != nil || !strings.Contains(cfg, "PATH") {
			t.Fatalf("config: %v %q", err, cfg)
		}
		img.Close()
	}
}

func TestNotAnImage(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.tar")
	_ = os.WriteFile(plain, tarOf(t, tarMember{"a.txt", []byte("a")}), 0o600)
	zipped := filepath.Join(dir, "image.tar.gz")
	_ = os.WriteFile(zipped, gz(t, tarOf(t, tarMember{"manifest.json", []byte("[]")})), 0o600)
	for _, p := range []string{plain, zipped, filepath.Join(dir, "missing.tar")} {
		if _, err := OpenImage(p, Options{}); err == nil {
			t.Errorf("%s: expected an error", p)
		}
	}
}

func zipOf(t *testing.T, members ...tarMember) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, m := range members {
		w, err := zw.Create(m.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(m.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestZipArchivesAreReadInside(t *testing.T) {
	data := zipOf(t,
		tarMember{"data/users.csv", []byte("a,b\n1,2\n")},
		tarMember{"inner.tar.gz", gz(t, tarOf(t, tarMember{"notes.txt", []byte("inner\n")}))},
		tarMember{"vendor/skip.txt", []byte("x\n")},
	)
	got, out := visit(t, "export.zip", data, Options{Exclude: []string{"vendor"}})
	want := []string{"export.zip!/data/users.csv", "export.zip!/inner.tar.gz!/notes.txt"}
	if fmt.Sprint(keysOf(got)) != fmt.Sprint(want) || got[want[0]] != "a,b\n1,2\n" {
		t.Fatalf("got %v", got)
	}
	if len(out.Skipped)+len(out.Unreadable) != 0 {
		t.Fatalf("outcome %+v", out)
	}
	got, _ = visit(t, "dump.tar", tarOf(t, tarMember{"export.zip", data}), Options{})
	if got["dump.tar!/export.zip!/data/users.csv"] != "a,b\n1,2\n" {
		t.Fatalf("zip inside tar: %v", keysOf(got))
	}
	_, out = visit(t, "big.zip", data, Options{Limits: Limits{MaxBytes: 64}})
	if len(out.Unreadable) != 1 || !strings.Contains(out.Unreadable[0].Reason, "max_archive_bytes") {
		t.Fatalf("byte limit: %+v", out)
	}
}

func TestUnopenedFormats(t *testing.T) {
	for name, want := range map[string]bool{"report.XLSX": true, "contract.pdf": true, "dump.7z": true, "app.db": true, "logo.png": false, "app.zip": false, "main.go": false} {
		if Unopened(name) != want {
			t.Errorf("Unopened(%q) = %v", name, !want)
		}
	}
}
