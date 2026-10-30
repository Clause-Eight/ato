package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/domain"
)

func scanRows(t *testing.T, path, text string, maxRows int) textResult {
	t.Helper()
	res, err := scanText(path, strings.NewReader(text), []detectors.Detector{sevens}, maxRows, false)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func byColumn(fs []domain.Finding) map[string][]domain.Finding {
	out := map[string][]domain.Finding{}
	for _, f := range fs {
		out[f.ColumnName] = append(out[f.ColumnName], f)
	}
	return out
}

func TestHeaderGivesEveryRowItsKeyword(t *testing.T) {
	var b strings.Builder
	b.WriteString("id,secret,note\n")
	for i := range 50 {
		fmt.Fprintf(&b, "%d,1234567,ref 7654327\n", i)
	}
	res := scanRows(t, "export/users.csv", b.String(), 0)
	cols := byColumn(res.findings)
	if len(cols["secret"]) != 50 || len(cols["note"]) != 50 {
		t.Fatalf("columns: %v", len(res.findings))
	}
	for _, f := range cols["secret"] {
		if f.Confidence != domain.High || f.Location.Line == 2 && f.Location.Column != 3 {
			t.Fatalf("a value under a keyword header must be raised: %+v", f)
		}
	}
	for _, f := range cols["note"] {
		if f.Confidence != domain.Medium {
			t.Fatalf("a keyword in another column must not raise this one: %+v", f)
		}
	}
	if len(res.fields) != 3 || res.fields[1].Name != "secret" || res.fields[1].Container != "users" || res.fields[1].Format != "csv" {
		t.Fatalf("header fields: %+v", res.fields)
	}
}

func TestDelimiterIsSniffed(t *testing.T) {
	for _, sep := range []string{";", "\t", "|"} {
		text := strings.Join([]string{"name", "secret"}, sep) + "\n" + strings.Join([]string{"x, y", "1234567"}, sep) + "\n"
		fs := scanRows(t, "a.csv", text, 0).findings
		if len(fs) != 1 || fs[0].ColumnName != "secret" || fs[0].Confidence != domain.High {
			t.Errorf("separator %q: %+v", sep, fs)
		}
	}
	fs := scanRows(t, "a.tsv", "secret\tother\n1234567\t7654327\n", 0).findings
	if len(fs) != 2 || fs[0].ColumnName != "secret" || fs[1].ColumnName != "other" {
		t.Fatalf("tsv: %+v", fs)
	}
}

func TestQuotedCellsAndMissingHeader(t *testing.T) {
	text := "secret,comment\n\"1234567\",\"a, b \"\"quoted\"\" and\nmore 7654327\"\n"
	fs := scanRows(t, "a.csv", text, 0).findings
	if len(fs) != 2 || fs[0].ColumnName != "secret" || fs[1].ColumnName != "comment" || fs[1].Location.Line != 3 {
		t.Fatalf("quoted cells spanning lines: %+v", fs)
	}
	fs = scanRows(t, "a.csv", "1,1234567\n2,7654327\n", 0).findings
	if len(fs) != 2 || fs[0].ColumnName != "#2" || fs[0].Location.Line != 1 {
		t.Fatalf("a first row of data is not a header: %+v", fs)
	}
}

func TestMaxRowsSamples(t *testing.T) {
	var b strings.Builder
	b.WriteString("secret\n")
	for range 10 {
		b.WriteString("1234567\n")
	}
	res := scanRows(t, "a.csv", b.String(), 4)
	if len(res.findings) != 4 || !res.sampled {
		t.Fatalf("got %d findings, sampled %v", len(res.findings), res.sampled)
	}
	if res := scanRows(t, "a.csv", b.String(), 10); res.sampled || len(res.findings) != 10 {
		t.Fatalf("a file with exactly max rows is not sampled: %d %v", len(res.findings), res.sampled)
	}
	if res := scanRows(t, "a.csv", b.String(), 0); res.sampled || len(res.findings) != 10 {
		t.Fatalf("0 reads every row: %d", len(res.findings))
	}
}

func TestJSONLinesKeysName(t *testing.T) {
	text := `{"id": 1, "customer": {"secret": "1234567", "tags": ["x", 7654327]}, "note": "1234567"}` + "\n" +
		`not json 1234567` + "\n"
	res := scanRows(t, "events.ndjson", text, 0)
	cols := byColumn(res.findings)
	if len(cols["customer.secret"]) != 1 || cols["customer.secret"][0].Confidence != domain.High {
		t.Fatalf("nested key: %+v", res.findings)
	}
	if len(cols["customer.tags"]) != 1 || len(cols["note"]) != 1 || cols["note"][0].Confidence != domain.Medium {
		t.Fatalf("array and plain keys: %+v", res.findings)
	}
	if len(cols[""]) != 1 {
		t.Fatalf("a line that is not JSON is scanned whole: %+v", res.findings)
	}
	names := map[string]string{}
	for _, f := range res.fields {
		names[f.Container+"."+f.Name] = f.Format
	}
	if names["customer.secret"] != "jsonl" || names["events.note"] != "jsonl" {
		t.Fatalf("keys: %+v", res.fields)
	}
}

func TestSQLValuesAreNamedByColumn(t *testing.T) {
	text := strings.Join([]string{
		"CREATE TABLE `people` (",
		"  `id` int NOT NULL,",
		"  `secret` varchar(20) DEFAULT NULL,",
		"  `note` text,",
		"  PRIMARY KEY (`id`)",
		");",
		"INSERT INTO `people` VALUES (1,'1234567','x'),(2,NULL,'7654327'),(3,'1234567','it''s, 7654327');",
		"INSERT INTO people (note, secret) VALUES",
		"  ('1234567', '7654327'),",
		"  ('multi",
		"line 1234567', NULL);",
		"COPY public.people (id, secret, note) FROM stdin;",
		"4\t1234567\t\\N",
		"\\.",
		"-- secret 1234567",
	}, "\n") + "\n"
	fs := scanRows(t, "dump.sql", text, 0).findings
	type got struct {
		line   int
		column string
		conf   domain.Confidence
	}
	var g []got
	for _, f := range fs {
		g = append(g, got{f.Location.Line, f.ColumnName, f.Confidence})
	}
	want := []got{
		{7, "secret", domain.High}, {7, "note", domain.Medium}, {7, "secret", domain.High}, {7, "note", domain.Medium},
		{9, "note", domain.Medium}, {9, "secret", domain.High},
		{11, "note", domain.Medium},
		{13, "secret", domain.High},
		{15, "", domain.High},
	}
	if fmt.Sprint(g) != fmt.Sprint(want) {
		t.Fatalf("got  %v\nwant %v", g, want)
	}
}

func TestLongInsertLineIsNotATableOfNumbers(t *testing.T) {
	var rows []string
	for i := range 2000 {
		rows = append(rows, fmt.Sprintf("(%d,'1234567','40%08d')", i, i))
	}
	fs := scanRows(t, "dump.sql", "INSERT INTO t (id, secret, phone) VALUES "+strings.Join(rows, ",")+";\n", 0).findings
	if len(fs) != 2000 {
		t.Fatalf("got %d findings", len(fs))
	}
	for _, f := range fs {
		if f.Confidence != domain.High || f.ColumnName != "secret" {
			t.Fatalf("%+v", f)
		}
	}
}

func BenchmarkScanCSV(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("id,first_name,last_name,email,phone,city,created_at,status\n")
	for i := range 20000 {
		fmt.Fprintf(&sb, "%d,Name%d,Surname%d,user%d@example.test,07%08d,City,2026-01-%02d,active\n", i, i, i, i, i, i%28+1)
	}
	body := sb.String()
	dets := detectors.All()
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		if _, err := scanText("bench.csv", strings.NewReader(body), dets, 0, false); err != nil {
			b.Fatal(err)
		}
	}
}
