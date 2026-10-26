package cli

import (
	"bytes"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/synth"
)

func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"nope"}, {"scan", "a", "b"}, {"scan", "--format", "xml", "."}, {"scan", "--min-confidence", "very", "."}, {"scan", "--include", "", "."}} {
		code, _, _ := run(args...)
		if code != ExitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, ExitUsage)
		}
	}
}

func TestVersionAndHelp(t *testing.T) {
	if code, out, _ := run("version"); code != 0 || !strings.HasPrefix(out, "pii-eu ") {
		t.Fatalf("version: %d %q", code, out)
	}
	if code, out, _ := run("help"); code != 0 || !strings.Contains(out, "Exit codes") {
		t.Fatalf("help: %d", code)
	}
}

func TestCleanFolderExitsZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("nothing personal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run("scan", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "No personal data found") {
		t.Fatalf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestUnknownDetectorInSettings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("[detectors]\nnot-a-detector = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run("scan", dir)
	if code != ExitUsage || !strings.Contains(errOut, "unknown detector") {
		t.Fatalf("exit %d err %q", code, errOut)
	}
}

func TestFlagsAfterPath(t *testing.T) {
	got := reorder([]string{".", "--format", "json", "--no-git", "--include=x"})
	want := "--format json --no-git --include=x ."
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v", got)
	}
}

func syntheticCard() string {
	body := "453201511283036"
	for c := '0'; c <= '9'; c++ {
		if v := body + string(c); detectors.Luhn(v) {
			return v
		}
	}
	return ""
}

func TestScanWritesEachFormatWithoutValues(t *testing.T) {
	dir := t.TempDir()
	card := syntheticCard()
	if err := os.WriteFile(filepath.Join(dir, "pay.txt"), []byte("card: "+card+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"text", "json", "sarif"} {
		out := filepath.Join(t.TempDir(), "report."+format)
		code, _, errOut := run("scan", "--no-git", "--format", format, "--output", out, "--show-values", dir)
		if code != ExitFindings {
			t.Fatalf("%s: exit %d (%s)", format, code, errOut)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), card) {
			t.Errorf("%s report file contains the raw value", format)
		}
	}
}

func TestBaselineReportsOnlyNewFindings(t *testing.T) {
	dir := t.TempDir()
	card := syntheticCard()
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(dir, "pay.txt"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("card: " + card + "\n")
	if code, out, errOut := run("baseline", "--no-git", dir); code != ExitClean || !strings.Contains(out, "1 finding") {
		t.Fatalf("baseline: %d %q %q", code, out, errOut)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".pii-eu-baseline.json"))
	if strings.Contains(string(data), card) {
		t.Fatal("baseline contains the value")
	}
	if code, out, _ := run("scan", "--no-git", dir); code != ExitClean || !strings.Contains(out, "1 in the baseline") {
		t.Fatalf("baselined scan: %d %q", code, out)
	}
	write("card: " + card + "\nother card: " + card + "\n")
	if code, _, _ := run("scan", "--no-git", dir); code != ExitFindings {
		t.Fatalf("a new finding should fail: %d", code)
	}
}

func TestScanStdin(t *testing.T) {
	Stdin = strings.NewReader("log line card=" + syntheticCard() + "\n")
	defer func() { Stdin = os.Stdin }()
	code, out, _ := run("scan", "-")
	if code != ExitFindings || !strings.Contains(out, "<stdin>") {
		t.Fatalf("stdin: %d %q", code, out)
	}
}

func TestStagedScansTheStagedVersion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	p := filepath.Join(dir, "pay.txt")
	_ = os.WriteFile(p, []byte("card: "+syntheticCard()+"\n"), 0o600)
	git("add", "pay.txt")
	_ = os.WriteFile(p, []byte("clean now\n"), 0o600)
	if code, _, errOut := run("scan", "--staged", dir); code != ExitFindings {
		t.Fatalf("staged content should be scanned: %d %s", code, errOut)
	}
	if code, _, _ := run("scan", "--staged", t.TempDir()); code != ExitUsage {
		t.Fatalf("--staged outside git should be a usage error, got %d", code)
	}
}

func TestDetectorReferenceIsUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "DETECTORS.md")
	if os.Getenv("PII_EU_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(DetectorTable()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != DetectorTable() {
		t.Fatal("docs/DETECTORS.md is out of date: run PII_EU_UPDATE_GOLDEN=1 go test ./internal/cli")
	}
}

func TestCoverageMatrixInReadmeIsUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	const start, end = "<!-- coverage:start -->\n", "<!-- coverage:end -->"
	i, j := strings.Index(text, start), strings.Index(text, end)
	if i < 0 || j < i {
		t.Fatal("README.md has no coverage markers")
	}
	want := text[:i+len(start)] + CoverageMatrix() + text[j:]
	if os.Getenv("PII_EU_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if text != want {
		t.Fatal("the coverage matrix in README.md is out of date: run PII_EU_UPDATE_GOLDEN=1 go test ./internal/cli")
	}
}

func TestCountriesFlag(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewSource(1))
	gen, _ := synth.For("ro-cnp")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("cnp: "+gen(r)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := run("scan", "--no-git", "--countries", "bg,hu", dir); code != ExitClean {
		t.Errorf("a Romanian number with only BG and HU selected: exit %d", code)
	}
	if code, _, _ := run("scan", "--no-git", "--countries", "cee", dir); code != ExitFindings {
		t.Errorf("the cee preset includes RO: exit %d", code)
	}
	if code, _, errOut := run("scan", "--no-git", "--countries", "xx", dir); code != ExitUsage || !strings.Contains(errOut, "unknown country") {
		t.Errorf("unknown country: exit %d, %q", code, errOut)
	}
}

func TestSpecialCategoryFieldsAndInventory(t *testing.T) {
	dir := t.TempDir()
	migration := "CREATE TABLE patients (\n  id bigint,\n  first_name text,\n  diagnosis text,\n  created_at timestamp\n);\n"
	if err := os.WriteFile(filepath.Join(dir, "001_patients.sql"), []byte(migration), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("scan", "--no-git", "--format", "json", dir)
	if code != ExitFindings || !strings.Contains(out, `"special_category": "health"`) || !strings.Contains(out, `"kind": "field"`) {
		t.Fatalf("scan: exit %d\n%s", code, out)
	}
	if strings.Contains(out, "first_name") {
		t.Error("only special-category fields are scan findings")
	}
	cfg := filepath.Join(dir, ".pii-eu.toml")
	if err := os.WriteFile(cfg, []byte("[detectors]\nspecial-category = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := run("scan", "--no-git", dir); code != ExitClean {
		t.Errorf("special-category = false must switch the findings off: exit %d", code)
	}
	code, out, _ = run("inventory", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "first_name,patients,name,,sql,001_patients.sql,3") || !strings.Contains(out, "diagnosis,patients,special-category,health,sql") {
		t.Fatalf("inventory: exit %d\n%s", code, out)
	}
	if strings.Contains(out, "created_at") {
		t.Error("fields without personal data belong out of the inventory")
	}
	if code, _, _ := run("inventory", "--format", "sarif", dir); code != ExitUsage {
		t.Errorf("inventory accepts only json and csv: exit %d", code)
	}
}

// The Terraform and SQL here are made up.
func TestTerraformStores(t *testing.T) {
	dir := t.TempDir()
	tf := `provider "aws" {
  region = "eu-central-1"
}

resource "aws_db_instance" "patients" {
  identifier = "patients"
}

resource "aws_s3_bucket" "site" {
  bucket = "example-site"
}
`
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("main.tf", tf)
	write("001_patients.sql", "CREATE TABLE patients (\n  id bigint,\n  full_name text\n);\n")
	code, out, errOut := run("scan", "--no-git", "--format", "json", dir)
	if code != ExitFindings || !strings.Contains(out, `"detector": "tf-unencrypted"`) || !strings.Contains(out, `"type": "aws_db_instance"`) || !strings.Contains(out, `"confidence": "high"`) {
		t.Fatalf("scan: exit %d %s\n%s", code, errOut, out)
	}
	if !strings.Contains(out, `"masked": "aws_s3_bucket.site"`) {
		t.Errorf("a bucket kept forever is a medium finding:\n%s", out)
	}
	write(".pii-eu.toml", "[detectors]\ntf-unencrypted = false\ntf-no-retention = false\n")
	if code, out, errOut := run("scan", "--no-git", dir); code != ExitClean {
		t.Errorf("checks switched off: exit %d %s %s", code, out, errOut)
	}
	code, out, _ = run("inventory", "--no-git", "--format", "json", dir)
	if code != ExitClean || !strings.Contains(out, `"stores": [`) || !strings.Contains(out, `"in_eea": "yes"`) || !strings.Contains(out, `"encrypted": "no"`) {
		t.Fatalf("inventory: exit %d\n%s", code, out)
	}
	code, out, _ = run("inventory", "--no-git", dir)
	if code != ExitClean || strings.Contains(out, "aws_") {
		t.Errorf("the CSV inventory keeps its columns: %s", out)
	}
	if _, out, _ := run("detectors"); !strings.Contains(out, "tf-non-eea-region") {
		t.Error("the detector list names the Terraform checks")
	}
}

func TestDeprecatedFlagWarns(t *testing.T) {
	deprecatedFlags["no-git"] = "--walk"
	defer delete(deprecatedFlags, "no-git")
	_, _, errOut := run("scan", "--no-git", t.TempDir())
	if !strings.Contains(errOut, "--no-git is deprecated") || !strings.Contains(errOut, "--walk") {
		t.Fatalf("no warning: %q", errOut)
	}
}

func TestMapAndDiff(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("services/clinic/db/001.sql", "CREATE TABLE patients (\n  first_name text,\n  diagnosis text\n);\n")
	write("services/billing/requirements.txt", "stripe==10.1.0\n")
	code, out, errOut := run("map", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "| `patients` | name | health |") || !strings.Contains(out, "| Stripe | payments |") {
		t.Fatalf("map: exit %d\n%s%s", code, out, errOut)
	}
	saved := filepath.Join(t.TempDir(), "map.json")
	if code, _, errOut := run("map", "--no-git", "--format", "json", "--output", saved, dir); code != ExitClean {
		t.Fatalf("map json: exit %d %s", code, errOut)
	}
	if code, out, _ := run("map", "--no-git", "--diff", saved, dir); code != ExitClean || !strings.Contains(out, "Nothing new") {
		t.Errorf("unchanged map: exit %d\n%s", code, out)
	}
	write("services/clinic/ai.py", "import openai\n")
	code, out, _ = run("map", "--no-git", "--diff", saved, dir)
	if code != ExitFindings || !strings.Contains(out, "services/clinic") || !strings.Contains(out, "OpenAI") {
		t.Errorf("new recipient: exit %d\n%s", code, out)
	}
	if code, _, _ := run("map", "--format", "sarif", dir); code != ExitUsage {
		t.Errorf("map accepts only markdown, json and html: exit %d", code)
	}
	if code, _, _ := run("map", "--diff", filepath.Join(dir, "missing.json"), dir); code != ExitUsage {
		t.Errorf("missing diff file: exit %d", code)
	}
}

func TestErasureAndRetentionFindings(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"db/001.sql":  "CREATE TABLE customers (\n  first_name text\n);\nCREATE TABLE orders (\n  customer_id bigint REFERENCES customers(id) ON DELETE CASCADE,\n  contact_email text\n);\nCREATE TABLE leads (\n  email text\n);\n",
		"app/repo.py": "def forget(pid):\n    db.execute(\"DELETE FROM customers WHERE id = %s\", pid)\n",
		"app/jobs.py": "def purge_old_leads():\n    pass\n",
	}
	for rel, text := range files {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code, out, _ := run("scan", "--no-git", dir); code != ExitClean {
		t.Fatalf("off by default: exit %d\n%s", code, out)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pii-eu.toml"), []byte("[detectors]\nno-erasure-path = true\nno-retention = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("scan", "--no-git", "--format", "json", dir)
	if code != ExitFindings {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, want := range []string{`"masked": "leads"`, `"no-erasure-path"`, `"no-retention"`, `"kind": "lifecycle"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if strings.Count(out, `"detector": "no-erasure-path"`) != 1 {
		t.Errorf("only leads lacks an erasure path (orders cascade from customers):\n%s", out)
	}
	code, out, _ = run("map", "--no-git", dir)
	if code != ExitClean || !strings.Contains(out, "anonymising routine") && !strings.Contains(out, "SQL DELETE") || !strings.Contains(out, "ON DELETE CASCADE") {
		t.Errorf("map should show erasure paths: exit %d\n%s", code, out)
	}
}
