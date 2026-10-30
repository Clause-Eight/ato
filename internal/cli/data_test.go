package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/synth"
)

func syntheticCNPs(t *testing.T, n int) []string {
	t.Helper()
	gen, ok := synth.For("ro-cnp")
	if !ok {
		t.Fatal("no ro-cnp generator")
	}
	r := rand.New(rand.NewSource(7))
	out := make([]string, n)
	for i := range out {
		out[i] = gen(r)
	}
	return out
}

func usersCSV(cnps []string) []byte {
	var b strings.Builder
	b.WriteString("id,cnp,diagnosis\n")
	for i, c := range cnps {
		fmt.Fprintf(&b, "%d,%s,none\n", i+1, c)
	}
	return []byte(b.String())
}

func tarBytes(t *testing.T, files map[string][]byte, order ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range order {
		body := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gzBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(data)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type jsonDoc struct {
	SchemaVersion string `json:"schema_version"`
	Summary       struct {
		FilesScanned int `json:"files_scanned"`
		Sampled      []struct {
			Path string `json:"path"`
		} `json:"sampled"`
		Columns []struct {
			Path       string `json:"path"`
			ColumnName string `json:"column_name"`
			Findings   int    `json:"findings"`
		} `json:"columns"`
	} `json:"summary"`
	Findings []struct {
		Detector   string   `json:"detector"`
		Path       string   `json:"path"`
		Line       int      `json:"line"`
		ColumnName string   `json:"column_name"`
		Confidence string   `json:"confidence"`
		Reasons    []string `json:"reasons"`
		Kind       string   `json:"kind"`
	} `json:"findings"`
}

func scanJSON(t *testing.T, args ...string) (int, jsonDoc, string) {
	t.Helper()
	code, out, errOut := run(append([]string{"scan", "--no-git", "--format", "json"}, args...)...)
	var doc jsonDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("exit %d: %v\n%s\n%s", code, err, out, errOut)
	}
	return code, doc, out
}

func TestCSVInsideATarballIsScannedByColumn(t *testing.T) {
	dir := t.TempDir()
	cnps := syntheticCNPs(t, 30)
	data := gzBytes(t, tarBytes(t, map[string][]byte{"data/users.csv": usersCSV(cnps), "README.txt": []byte("export\n")}, "README.txt", "data/users.csv"))
	if err := os.WriteFile(filepath.Join(dir, "dump.tar.gz"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	code, doc, out := scanJSON(t, dir)
	if code != ExitFindings || doc.SchemaVersion != "1.3" {
		t.Fatalf("exit %d\n%s", code, out)
	}
	values, fields := 0, 0
	for _, f := range doc.Findings {
		if f.Path != "dump.tar.gz!/data/users.csv" {
			t.Fatalf("path %q", f.Path)
		}
		switch {
		case f.Kind == "field":
			fields++
			if f.ColumnName != "diagnosis" {
				t.Errorf("special-category column: %+v", f)
			}
		case f.Detector == "ro-cnp":
			values++
			if f.ColumnName != "cnp" || f.Confidence != "high" {
				t.Errorf("a cnp column raises every row: %+v", f)
			}
		}
	}
	if values != len(cnps) || fields != 1 || doc.Summary.FilesScanned != 2 {
		t.Fatalf("values %d fields %d scanned %d\n%s", values, fields, doc.Summary.FilesScanned, out)
	}
	if len(doc.Summary.Columns) != 2 || doc.Summary.Columns[1].ColumnName != "cnp" || doc.Summary.Columns[1].Findings != len(cnps) {
		t.Fatalf("columns: %+v", doc.Summary.Columns)
	}
	for _, c := range cnps {
		if strings.Contains(out, c) {
			t.Fatal("the report contains a value")
		}
	}
}

func TestMaxRowsFlagAndSetting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), usersCSV(syntheticCNPs(t, 20)), 0o600); err != nil {
		t.Fatal(err)
	}
	count := func(doc jsonDoc) int {
		n := 0
		for _, f := range doc.Findings {
			if f.Detector == "ro-cnp" {
				n++
			}
		}
		return n
	}
	_, doc, _ := scanJSON(t, "--max-rows", "5", dir)
	if count(doc) != 5 || len(doc.Summary.Sampled) != 1 || doc.Summary.Sampled[0].Path != "users.csv" {
		t.Fatalf("--max-rows 5: %d %+v", count(doc), doc.Summary.Sampled)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("max_rows = 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, doc, _ = scanJSON(t, dir); count(doc) != 3 {
		t.Fatalf("max_rows = 3: %d", count(doc))
	}
	if _, doc, _ = scanJSON(t, "--max-rows", "0", dir); count(doc) != 20 || len(doc.Summary.Sampled) != 0 {
		t.Fatalf("--max-rows 0 reads every row: %d", count(doc))
	}
	if code, _, _ := run("scan", "--max-rows", "-3", dir); code != ExitUsage {
		t.Fatalf("negative --max-rows: exit %d", code)
	}
}

// dockerSave writes a synthetic `docker save` tarball: a base layer with a config file and a data export, and
// a top layer that deletes the export.
func dockerSave(t *testing.T, dir string, cnp string) string {
	t.Helper()
	base := tarBytes(t, map[string][]byte{
		"etc/app/config.yml":   []byte("owner_cnp: " + cnp + "\n"),
		"srv/export/users.csv": usersCSV([]string{cnp}),
		"usr/share/doc/a.txt":  []byte("cnp: " + cnp + "\n"),
	}, "etc/app/config.yml", "srv/export/users.csv", "usr/share/doc/a.txt")
	top := tarBytes(t, map[string][]byte{"srv/export/.wh.users.csv": nil}, "srv/export/.wh.users.csv")
	l1, l2 := strings.Repeat("1", 64), strings.Repeat("2", 64)
	manifest := fmt.Sprintf(`[{"Config":"%s.json","RepoTags":["app:test"],"Layers":["%s/layer.tar","%s/layer.tar"]}]`, strings.Repeat("c", 64), l1, l2)
	img := tarBytes(t, map[string][]byte{
		l1 + "/layer.tar":                 gzBytes(t, base),
		l2 + "/layer.tar":                 top,
		strings.Repeat("c", 64) + ".json": []byte(`{"config":{"Env":["A=1"]}}`),
		"manifest.json":                   []byte(manifest),
	}, l1+"/layer.tar", l2+"/layer.tar", strings.Repeat("c", 64)+".json", "manifest.json")
	p := filepath.Join(dir, "image.tar")
	if err := os.WriteFile(p, img, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScanSavedImage(t *testing.T) {
	dir := t.TempDir()
	p := dockerSave(t, dir, syntheticCNPs(t, 1)[0])
	code, doc, out := scanJSON(t, "--image", p)
	if code != ExitFindings {
		t.Fatalf("exit %d\n%s", code, out)
	}
	prefix := filepath.ToSlash(p) + "!/111111111111/"
	got := map[string]string{}
	for _, f := range doc.Findings {
		if f.Detector == "ro-cnp" {
			got[strings.TrimPrefix(f.Path, prefix)] = strings.Join(f.Reasons, "; ")
		}
	}
	if len(got) != 2 || !strings.Contains(got["srv/export/users.csv"], "deleted in later layer 222222222222") || strings.Contains(got["etc/app/config.yml"], "later layer") {
		t.Fatalf("image findings: %v\n%s", got, out)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("image_exclude = []\nexclude = [\"srv/\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, doc, _ = scanJSON(t, "--image", p)
	paths := map[string]bool{}
	for _, f := range doc.Findings {
		paths[strings.TrimPrefix(f.Path, prefix)] = true
	}
	if !paths["usr/share/doc/a.txt"] || paths["srv/export/users.csv"] {
		t.Fatalf("image_exclude = [] scans OS folders and exclude still applies: %v", paths)
	}
	if code, _, errOut := run("scan", "--image", p, dir); code != ExitUsage {
		t.Fatalf("a path and --image: exit %d %s", code, errOut)
	}
	plain := filepath.Join(dir, "plain.tar")
	_ = os.WriteFile(plain, tarBytes(t, map[string][]byte{"a.txt": []byte("a")}, "a.txt"), 0o600)
	if code, _, errOut := run("scan", "--image", plain); code != ExitUsage || !strings.Contains(errOut, "not a saved container image") {
		t.Fatalf("not an image: exit %d %s", code, errOut)
	}
}

func TestArchiveLimitMakesTheScanIncomplete(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zeros.txt.gz"), gzBytes(t, make([]byte, 4<<20)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("max_archive_bytes = 1048576\nmax_file_size = 104857600\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("scan", "--no-git", dir)
	if code != ExitIncomplete || !strings.Contains(out, "max_archive_bytes") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

// TestPartlyReadFilesMakeTheScanIncomplete: a clean result must mean everything that may hold personal data was
// read. A document format pii-eu does not open, or a data file cut at max_rows, exits 3 unless allow_partial.
func TestPartlyReadFilesMakeTheScanIncomplete(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "export.xlsx"), []byte("PK\x03\x04\x00\x00binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), []byte("\x89PNG\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("scan", "--no-git", dir)
	if code != ExitIncomplete || !strings.Contains(out, "did not read all of export.xlsx") || strings.Contains(out, "logo.png") {
		t.Fatalf("office document: exit %d\n%s", code, out)
	}
	if code, _, _ := run("scan", "--no-git", "--allow-partial", dir); code != ExitClean {
		t.Fatalf("--allow-partial: exit %d", code)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("allow_partial = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := run("scan", "--no-git", dir); code != ExitClean {
		t.Fatalf("allow_partial = true: exit %d", code)
	}

	rows := t.TempDir()
	var csv strings.Builder
	csv.WriteString("id,plan\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&csv, "%d,pro\n", i)
	}
	if err := os.WriteFile(filepath.Join(rows, "plans.csv"), []byte(csv.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := run("scan", "--no-git", "--max-rows", "5", rows); code != ExitIncomplete {
		t.Fatalf("cut at --max-rows: exit %d", code)
	}
	if code, _, _ := run("scan", "--no-git", "--max-rows", "0", rows); code != ExitClean {
		t.Fatalf("every row read: exit %d", code)
	}
}
